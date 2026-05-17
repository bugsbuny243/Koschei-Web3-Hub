extends Node
class_name GodotWeb3Bridge

signal request_succeeded(endpoint: String, payload: Dictionary)
signal request_failed(endpoint: String, error_message: String)

@export var base_url := "http://localhost:4000"
var _http := HTTPRequest.new()

func _ready() -> void:
	add_child(_http)
	_http.request_completed.connect(_on_completed)

func create_wallet(player: String, wallet_address: String) -> void:
	if player.is_empty() or wallet_address.is_empty():
		emit_signal("request_failed", "/api/wallet/create", "Invalid input")
		return
	_post("/api/wallet/create", {"player": player, "walletAddress": wallet_address})

func create_profile(player: String, username: String) -> void:
	if username.length() < 3:
		emit_signal("request_failed", "/api/profile/create", "Username too short")
		return
	_post("/api/profile/create", {"player": player, "username": username})

func mint_asset(to: String, asset_type: String, godot_id: String, properties: String) -> void:
	_post("/api/asset/mint", {"to": to, "assetType": asset_type, "godotId": godot_id, "properties": properties})

func add_experience(player: String, amount: int) -> void:
	if amount <= 0:
		emit_signal("request_failed", "/api/player/experience", "Amount must be positive")
		return
	_post("/api/player/experience", {"player": player, "amount": amount})

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
