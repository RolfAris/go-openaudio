#################################################
###       OpenAudio Configuration Options     ###
#################################################

[openaudio.version]
tag = "{{ .OpenAudio.Version.Tag }}"
git_sha = "{{ .OpenAudio.Version.GitSHA }}"

[openaudio.eth]
rpcurl = "{{ .OpenAudio.Eth.RpcURL }}"
registryaddress = "{{ .OpenAudio.Eth.RegistryAddress }}"

[openaudio.db]
postgres_dsn = "{{ .OpenAudio.DB.PostgresDSN }}"

[openaudio.blob]
blob_store_dsn = "{{ .OpenAudio.Blob.BlobStoreDSN }}"
move_from_blob_store_dsn = "{{ .OpenAudio.Blob.MoveFromBlobStoreDSN }}"

[openaudio.operator]
endpoint = "{{ .OpenAudio.Operator.Endpoint }}"

[openaudio.server]
port = "{{ .OpenAudio.Server.Port }}"
https_port = "{{ .OpenAudio.Server.HTTPSPort }}"
hostname = "{{ .OpenAudio.Server.Hostname }}"
h2c = "{{ .OpenAudio.Server.H2C }}"

[openaudio.server.tls]
enabled = "{{ .OpenAudio.Server.TLS.Enabled }}"
self_signed = "{{ .OpenAudio.Server.TLS.SelfSigned }}"
cert_dir = "{{ .OpenAudio.Server.TLS.CertDir }}"
cache_dir = "{{ .OpenAudio.Server.TLS.CacheDir }}"

[openaudio.server.console]
enabled = "{{ .OpenAudio.Server.Console.Enabled }}"
subroute = "{{ .OpenAudio.Server.Console.SubRoute }}"

[openaudio.server.socket]
enabled = "{{ .OpenAudio.Server.Socket.Enabled }}"
path = "{{ .OpenAudio.Server.Socket.Path }}"
