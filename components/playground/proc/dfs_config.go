package proc

func applyS3DFSConfig(config map[string]any, s3 CSEOptions, prefix string) {
	if config == nil {
		return
	}
	if prefix != "" {
		config["dfs.prefix"] = prefix
	}
	config["dfs.s3-endpoint"] = s3.S3Endpoint
	config["dfs.s3-key-id"] = s3.AccessKey
	config["dfs.s3-secret-key"] = s3.SecretKey
	config["dfs.s3-bucket"] = s3.Bucket
	config["dfs.s3-region"] = "local"
}
