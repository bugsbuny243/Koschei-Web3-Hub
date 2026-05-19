extends Node
class_name GodotWeb3Bridge

signal request_succeeded(endpoint: String, payload: Dictionary)
signal request_failed(endpoint: String, error_message: String)

@export var base_url := "http://localhost:4000"
var _http := HTTPRequest.new()

func _ready() -> void:
	add_child(_http)
	_http.request_completed.connect(_on_completed)

func create_item_metadata(project_id: String, payload: Dictionary) -> void:
	if project_id.is_empty():
		emit_signal("request_failed", "/api/game-factory/projects/[id]/web3-package", "Project id is required")
		return
	_post("/api/game-factory/projects/%s/web3-package" % project_id, payload)

func generate_adapter_config(project_id: String, payload: Dictionary) -> void:
	if project_id.is_empty():
		emit_signal("request_failed", "/api/game-factory/projects/[id]/web3-package", "Project id is required")
		return
	_post("/api/game-factory/projects/%s/web3-package" % project_id, payload)

func export_readiness_package(project_id: String) -> void:
	if project_id.is_empty():
		emit_signal("request_failed", "/api/game-factory/projects/[id]/web3-package", "Project id is required")
		return
	_post("/api/game-factory/projects/%s/web3-package" % project_id, {})

func _post(endpoint: String, body: Dictionary) -> void:
	var headers = ["Content-Type: application/json"]
	var err = _http.request(base_url + endpoint, headers, HTTPClient.METHOD_POST, JSON.stringify(body))
	if err != OK:
		emit_signal("request_failed", endpoint, "Network error: %s" % str(err))

func _on_completed(_result: int, response_code: int, _headers: PackedStringArray, body: PackedByteArray) -> void:
	var parsed = JSON.parse_string(body.get_string_from_utf8())
	if response_code >= 200 and response_code < 300:
		emit_signal("request_succeeded", "unknown", parsed)
	else:
		emit_signal("request_failed", "unknown", str(parsed))
