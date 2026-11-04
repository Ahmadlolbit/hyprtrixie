package editors

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"slices"
	"time"
)

const (
	Publisher = "hyprtrixie"
	Name      = "themes"
	manifest  = `<?xml version="1.0" encoding="utf-8"?>
<PackageManifest Version="2.0.0" xmlns="http://schemas.microsoft.com/developer/vsx-schema/2011">
  <Metadata>
    <Identity Language="en-US" Id="themes" Version="VERSION" Publisher="hyprtrixie"/>
    <DisplayName>hyprtrixie themes</DisplayName>
    <Description xml:space="preserve">The colours of every hyprtrixie theme</Description>
    <Categories>Themes</Categories>
    <Properties>
      <Property Id="Microsoft.VisualStudio.Code.Engine" Value="^1.70.0"/>
    </Properties>
  </Metadata>
  <Installation>
    <InstallationTarget Id="Microsoft.VisualStudio.Code"/>
  </Installation>
  <Dependencies/>
  <Assets>
    <Asset Type="Microsoft.VisualStudio.Code.Manifest" Path="extension/package.json" Addressable="true"/>
  </Assets>
</PackageManifest>
`
	contentTypes = `<?xml version="1.0" encoding="utf-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension=".json" ContentType="application/json"/>
  <Default Extension=".vsixmanifest" ContentType="text/xml"/>
</Types>
`
)

type Theme struct {
	Label string `json:"label"`
	UI    string `json:"uiTheme"`
	Path  string `json:"path"`
	file  string
	data  []byte
}

type contributes struct {
	Themes []Theme `json:"themes"`
}

type extension struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description"`
	Publisher   string            `json:"publisher"`
	Version     string            `json:"version"`
	Engines     map[string]string `json:"engines"`
	Categories  []string          `json:"categories"`
	Contributes contributes       `json:"contributes"`
}

func NewTheme(label, mode, file string, data []byte) Theme {
	ui := "vs-dark"
	if mode == "light" {
		ui = "vs"
	}
	return Theme{Label: label, UI: ui, Path: "./themes/" + file, file: file, data: data}
}

func packageJSON(version string, themes []Theme) ([]byte, error) {
	return json.MarshalIndent(extension{
		Name: Name, DisplayName: "hyprtrixie themes", Description: "The colours of every hyprtrixie theme",
		Publisher: Publisher, Version: version,
		Engines: map[string]string{"vscode": "^1.70.0"}, Categories: []string{"Themes"},
		Contributes: contributes{themes},
	}, "", "  ")
}

func zipped(files map[string][]byte) ([]byte, error) {
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)})
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(files[name]); err != nil {
			return nil, err
		}
	}
	err := writer.Close()
	return out.Bytes(), err
}

func VSIX(version string, themes []Theme) ([]byte, error) {
	described, err := packageJSON(version, themes)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{
		"[Content_Types].xml":    []byte(contentTypes),
		"extension.vsixmanifest": bytes.ReplaceAll([]byte(manifest), []byte("VERSION"), []byte(version)),
		"extension/package.json": described,
	}
	for _, theme := range themes {
		files["extension/themes/"+theme.file] = theme.data
	}
	return zipped(files)
}

func Fingerprint(themes []Theme) string {
	sum := fnv.New128a()
	for _, theme := range themes {
		fmt.Fprintf(sum, "%s\x00%s\x00%s\x00%d\x00", theme.Label, theme.UI, theme.file, len(theme.data))
		sum.Write(theme.data)
	}
	return hex.EncodeToString(sum.Sum(nil))
}
