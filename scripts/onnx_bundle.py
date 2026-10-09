"""Read ONNX external-data declarations without loading tensor payloads.

Used before quantizer publication, including controlled converters when the
optional ONNX Python package is absent. Runtime readiness is checked in Go.
"""
from pathlib import PurePosixPath


def external_locations(path):
    refs = set()
    with open(path, 'rb') as stream:
        stream.seek(0, 2)
        size = stream.tell()

        def fields(start, end):
            pos = start

            def varint():
                nonlocal pos
                value = 0
                for shift in range(0, 70, 7):
                    if pos >= end:
                        raise ValueError('truncated ONNX protobuf')
                    stream.seek(pos)
                    raw = stream.read(1)
                    if not raw:
                        raise ValueError('truncated ONNX protobuf')
                    pos += 1
                    byte = raw[0]
                    if shift == 63 and byte > 1:
                        raise ValueError('ONNX varint overflow')
                    value |= (byte & 127) << shift
                    if byte < 128:
                        return value
                raise ValueError('ONNX varint overflow')

            while pos < end:
                tag = varint()
                number, wire = tag >> 3, tag & 7
                if number == 0:
                    raise ValueError('invalid ONNX protobuf field')
                value, first, last = 0, pos, pos
                if wire == 0:
                    value = varint()
                elif wire in (1, 5):
                    first = pos
                    pos += 8 if wire == 1 else 4
                    last = pos
                elif wire == 2:
                    length = varint()
                    first = pos
                    pos += length
                    last = pos
                else:
                    raise ValueError('unsupported ONNX wire type')
                if pos > end:
                    raise ValueError('truncated ONNX protobuf')
                yield number, wire, first, last, value

        def tensor(start, end):
            external, location = False, ''
            for number, wire, first, last, value in fields(start, end):
                if number == 14 and wire == 0:
                    external = value == 1
                if number == 13 and wire == 2:
                    entry = {}
                    for key, ew, a, b, _ in fields(first, last):
                        if ew != 2 or b-a > 4096:
                            raise ValueError('invalid external tensor metadata')
                        stream.seek(a)
                        entry[key] = stream.read(b-a).decode('utf-8')
                    if entry.get(1) == 'location':
                        location = entry.get(2, '')
            if external or location:
                target = PurePosixPath(location)
                if not location or target.is_absolute() or '..' in target.parts or target == PurePosixPath('.'):
                    raise ValueError('unsafe or missing ONNX external location')
                refs.add(target.as_posix())

        def graph(start, end, depth=0):
            if depth > 32:
                raise ValueError('ONNX graph nesting too deep')
            for number, wire, first, last, _ in fields(start, end):
                if wire != 2:
                    continue
                if number == 5:
                    tensor(first, last)
                if number == 1:
                    for key, nw, a, b, _ in fields(first, last):
                        if key != 5 or nw != 2:
                            continue
                        for attr, aw, x, y, _ in fields(a, b):
                            if aw != 2:
                                continue
                            if attr in (5, 10):
                                tensor(x, y)
                            if attr in (6, 11):
                                graph(x, y, depth+1)

        found = False
        for number, wire, first, last, _ in fields(0, size):
            if number == 7 and wire == 2:
                found = True
                graph(first, last)
        if not found:
            raise ValueError('ONNX graph missing')
    return sorted(refs)
