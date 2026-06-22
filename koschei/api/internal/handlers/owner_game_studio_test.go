package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestOwnerGameSlug(t *testing.T) {
	if got := ownerGameSlug("Çılgın Uzay 2049!"); got != "cilgin-uzay-2049" {
		t.Fatalf("ownerGameSlug() = %q", got)
	}
	if got := ownerGameSlug("123 Arena"); got != "game-123-arena" {
		t.Fatalf("numeric slug = %q", got)
	}
}

func TestBuildOwnerExpoGameBundleContainsPlayableProject(t *testing.T) {
	t.Setenv("ANDROID_PLAY_PACKAGE_NAME", "com.koschei.playgame")
	spec := gameSpec{
		GameType: "lane runner",
		Theme: "neon space",
		Player: "pilot",
		Enemies: []string{"meteor", "drone"},
		Collectibles: []string{"crystal"},
		WinCondition: "Reach 100 points",
		TargetPlatforms: []string{"android"},
	}
	bundle, err := buildOwnerExpoGameBundle("Neon Runner", "project-123", "android_game", spec)
	if err != nil {
		t.Fatalf("buildOwnerExpoGameBundle() error = %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	files := map[string]string{}
	for _, file := range reader.File {
		body, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		raw, err := io.ReadAll(body)
		_ = body.Close()
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}
		files[file.Name] = string(raw)
	}
	for _, name := range []string{
		"neon-runner/App.js",
		"neon-runner/package.json",
		"neon-runner/app.json",
		"neon-runner/eas.json",
		"neon-runner/game-spec.json",
		"neon-runner/README.md",
	} {
		if _, ok := files[name]; !ok {
			t.Fatalf("bundle missing %s", name)
		}
	}
	app := files["neon-runner/App.js"]
	if !strings.Contains(app, `const TITLE = "Neon Runner";`) || !strings.Contains(app, "setInterval") {
		t.Fatalf("App.js is not a generated playable project: %s", app)
	}
	if strings.Contains(app, "__TITLE__") || strings.Contains(app, "__PROJECT_ID__") {
		t.Fatal("template placeholders were not replaced")
	}
	var appConfig struct {
		Expo struct {
			Android struct {
				Package string `json:"package"`
				VersionCode int `json:"versionCode"`
			} `json:"android"`
		} `json:"expo"`
	}
	if err := json.Unmarshal([]byte(files["neon-runner/app.json"]), &appConfig); err != nil {
		t.Fatalf("app.json invalid: %v", err)
	}
	if appConfig.Expo.Android.Package != "com.koschei.playgame" || appConfig.Expo.Android.VersionCode <= 0 {
		t.Fatalf("unexpected Android config: %#v", appConfig.Expo.Android)
	}
	if !strings.Contains(files["neon-runner/eas.json"], `"autoIncrement": true`) {
		t.Fatal("EAS production profile does not auto-increment version code")
	}
}
