#################################################
###       OpenAudio Configuration Options     ###
#################################################

[openaudio.version]
tag = "{{ .OpenAudio.Version.Tag }}"
git_sha = "{{ .OpenAudio.Version.GitSHA }}"

[openaudio.eth]
rpc_url = "{{ .OpenAudio.Eth.RpcURL }}"
registry_address = "{{ .OpenAudio.Eth.RegistryAddress }}"

[openaudio.db]
# the embedded postgres store 
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

[openaudio.server.echo]
ip_rate_limit = "{{ .OpenAudio.Server.Echo.IPRateLimit }}"
request_timeout = "{{ .OpenAudio.Server.Echo.RequestTimeout }}"

#############################################
###       OpenAudio Logging Options       ###
#############################################

[openaudio.logger.level]
level = "info"                   # debug, info, warn, error

[openaudio.logger]
development = false
encoding = "json"                  # "json" or "console"
disable_caller = false
disable_stacktrace = false
output_paths = ["stdout"]
error_output_paths = ["stderr"]

[openaudio.logger.encoder_config]
message_key = "msg"
level_key = "level"
time_key = "ts"
name_key = "logger"
caller_key = "caller"
stacktrace_key = "stacktrace"
line_ending = "\n"
encode_level = "lowercase"
encode_time = "iso8601"
encode_duration = "string"
encode_caller = "short"
