"""bge-m3 fp32 → int8 동적 양자화 (1회성 오프라인 변환)."""
import os, shutil, sys
import onnx
from onnxruntime.quantization import quantize_dynamic, QuantType

src_dir = os.path.expanduser("~/.local/share/canopy/models/bge-m3")
dst_dir = os.path.expanduser("~/.local/share/canopy/models/bge-m3-int8")
os.makedirs(dst_dir, exist_ok=True)

quantize_dynamic(
    model_input=os.path.join(src_dir, "model.onnx"),
    model_output=os.path.join(dst_dir, "model.onnx"),
    weight_type=QuantType.QInt8,
    use_external_data_format=True,  # 결과가 2GB 미만이어도 안전
    extra_options={"MatMulConstBOnly": True, "DefaultTensorType": onnx.TensorProto.FLOAT},
)

for f in ["config.json", "tokenizer.json", "tokenizer_config.json", "special_tokens_map.json"]:
    shutil.copy2(os.path.join(src_dir, f), os.path.join(dst_dir, f))

total = sum(os.path.getsize(os.path.join(dst_dir, f)) for f in os.listdir(dst_dir))
print(f"done: {dst_dir}  total {total/1e6:.0f} MB")
