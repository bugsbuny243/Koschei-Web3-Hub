from pathlib import Path

js_path = Path("public/js/owner-control-center.js")
html_path = Path("public/owner-production.html")
text = js_path.read_text(encoding="utf-8")

simple_replacements = {
    "Tara · açıkla · görsel üret": "Tara · açıkla · listele",
    "Adres gir, tam tara, kanıtı gör, görseli üret.": "Adres gir, tam tara, holder listesini ve kanıtı gör.",
    "Tam Tara + Görsel Üret": "Tam Tara + Listele",
    "Tam Radar raporu ve görsel hazır.": "Holder listesi ve açıklama hazır.",
}
for old, new in simple_replacements.items():
    if old not in text:
        raise SystemExit(f"missing expected text: {old}")
    text = text.replace(old, new)

narrative = "<p class=\"muted\" style=\"font-size:15px;line-height:1.7\">${esc(d.narrative||f.verdict||'Kanıt özeti üretilemedi.')}</p>"
if narrative not in text:
    raise SystemExit("narrative block not found")
text = text.replace(narrative, "", 1)

visual_block = "<div class=\"section-gap\" style=\"display:flex;gap:8px;flex-wrap:wrap\"><button class=\"btn primary\" data-download-poster type=\"button\">Holder Görselini İndir</button><button class=\"btn\" data-copy-summary type=\"button\">Açıklamayı Kopyala</button></div><canvas data-radar-poster width=\"1200\" height=\"1900\" style=\"width:min(100%,700px);display:block;margin:14px auto;border:1px solid #1de6c833;border-radius:18px;background:#02070d\"></canvas>${holderIntelligenceHTML(holders)}"
list_first_block = "${holderIntelligenceHTML(holders)}<details class=\"owner-details section-gap\" open><summary><span><b>Koschei açıklaması</b><small>Holder listesindeki miktar, yüzde, değer ve hareketlerin ne anlattığını açıklar.</small></span><span>⌄</span></summary><p class=\"muted section-gap\" style=\"font-size:15px;line-height:1.7\">${esc(d.narrative||f.verdict||'Kanıt özeti üretilemedi.')}</p></details><div class=\"section-gap\" style=\"display:flex;gap:8px;flex-wrap:wrap\"><button class=\"btn\" data-copy-summary type=\"button\">Açıklamayı Kopyala</button></div>"
if visual_block not in text:
    raise SystemExit("visual-first block not found")
text = text.replace(visual_block, list_first_block, 1)

if "Holder Görselini İndir" in text:
    raise SystemExit("visual download remains in primary report")
if text.index("holderIntelligenceHTML(holders)") > text.index("Koschei açıklaması"):
    raise SystemExit("holder list is not before explanation")

js_path.write_text(text, encoding="utf-8")

html = html_path.read_text(encoding="utf-8")
if "/js/owner-control-center.js?v=9" not in html:
    raise SystemExit("owner JS cache version v9 not found")
html = html.replace("/js/owner-control-center.js?v=9", "/js/owner-control-center.js?v=10", 1)
html_path.write_text(html, encoding="utf-8")
