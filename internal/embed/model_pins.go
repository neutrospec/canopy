package embed

// Pinned repository revision and artifact digests, obtained from the upstream
// Hugging Face model API (LFS SHA256) and revision-specific JSON on 2026-10-10.
// Updating these pins is an explicit source change, never resolve/main.
func defaultManifest() *Manifest {
	return &Manifest{Version: 1, Repository: ModelRepository, Revision: ModelRevision, Kind: "onnx-external", Pooling: MeanPooling, Dimension: Dimension, Files: map[string]Artifact{
		"model.onnx":              {SHA256: "ef046f21faf5aa86bcff05b6fb25c34ef9b163ee86053c147c6434160f13a914", Size: 80311},
		"model.onnx.data":         {SHA256: "89c101f1180cbb4dd52a109c19ac3b015475f853b0b29fad715cd992e405ee1d", Size: 2266886160},
		"config.json":             {SHA256: "685b51555ab2f081f2085a8e3b12df63cd7647e22563091c6473cb1e3a198fdc", Size: 714},
		"tokenizer.json":          {SHA256: "249df0778f236f6ece390de0de746838ef25b9d6954b68c2ee71249e0a9d8fd4", Size: 17082799},
		"tokenizer_config.json":   {SHA256: "7e4c1cc848840aeccdd763458c18dd525eb0f795c992e00ebe9c28554e7db2d4", Size: 1173},
		"special_tokens_map.json": {SHA256: "8c785abebea9ae3257b61681b4e6fd8365ceafde980c21970d001e834cf10835", Size: 964},
	}}
}
