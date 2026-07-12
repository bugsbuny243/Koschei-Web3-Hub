from pathlib import Path


def remove_go_function(text: str, signature: str) -> tuple[str, bool]:
    start = text.find(signature)
    if start < 0:
        return text, False
    brace = text.find('{', start)
    if brace < 0:
        raise SystemExit(f'malformed function: {signature}')
    depth = 0
    in_string = False
    escape = False
    i = brace
    while i < len(text):
        ch = text[i]
        if in_string:
            if escape:
                escape = False
            elif ch == '\\':
                escape = True
            elif ch == '"':
                in_string = False
        else:
            if ch == '"':
                in_string = True
            elif ch == '{':
                depth += 1
            elif ch == '}':
                depth -= 1
                if depth == 0:
                    end = i + 1
                    while end < len(text) and text[end] in '\r\n':
                        end += 1
                    return text[:start] + text[end:], True
        i += 1
    raise SystemExit(f'unclosed function: {signature}')


path = Path('internal/handlers/owner.go')
text = path.read_text()
text, changed = remove_go_function(text, 'func (h *Handler) ShopierWebhook(')
if changed:
    path.write_text(text)
