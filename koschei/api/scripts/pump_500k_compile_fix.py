from pathlib import Path

path = Path("internal/services/pumpportal_radar_adapter.go")
text = path.read_text()
text = text.replace('\t"fmt"\n', '', 1)
text = text.replace('eventID, err := a.Store.InsertEvent(ctx, SecurityRadarEventRecord{', '_, err := a.Store.InsertEvent(ctx, SecurityRadarEventRecord{', 1)
path.write_text(text)
