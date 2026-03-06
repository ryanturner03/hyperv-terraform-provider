package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// buildCreateISOStdinData builds a JSON payload of filename -> base64-encoded content
// to be passed via stdin to the ISO creation script.
func buildCreateISOStdinData(files map[string]string) string {
	payload := make(map[string]string, len(files))
	for name, content := range files {
		payload[name] = base64.StdEncoding.EncodeToString([]byte(content))
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

// buildCreateISOScript builds a PowerShell script that reads file data from stdin (as JSON
// with base64-encoded values), writes them to a temp directory, and creates an ISO.
// File contents are passed via stdin to avoid Windows command-line length limits.
func buildCreateISOScript(opts ISOOptions) string {
	var sb strings.Builder
	sb.WriteString("$ErrorActionPreference = 'Stop'\n")
	sb.WriteString("$tempDir = Join-Path $env:TEMP ([System.Guid]::NewGuid().ToString())\n")
	sb.WriteString("New-Item -ItemType Directory -Path $tempDir -Force | Out-Null\n")
	sb.WriteString("try {\n")

	// Read file data from stdin as JSON {filename: base64content}
	sb.WriteString("  $jsonInput = @($input) -join ''\n")
	sb.WriteString("  $files = $jsonInput | ConvertFrom-Json\n")
	sb.WriteString("  foreach ($prop in $files.PSObject.Properties) {\n")
	sb.WriteString("    $fp = Join-Path $tempDir $prop.Name\n")
	sb.WriteString("    $fd = Split-Path -Parent $fp\n")
	sb.WriteString("    if ($fd -ne $tempDir) { New-Item -ItemType Directory -Path $fd -Force | Out-Null }\n")
	sb.WriteString("    [System.IO.File]::WriteAllBytes($fp, [System.Convert]::FromBase64String($prop.Value))\n")
	sb.WriteString("  }\n")

	// Create the ISO using IMAPI2
	sb.WriteString(fmt.Sprintf("  $isoPath = %s\n", EscapePSString(opts.Path)))
	sb.WriteString("  $parentDir = Split-Path -Parent $isoPath\n")
	sb.WriteString("  if ($parentDir -and -not (Test-Path $parentDir)) {\n")
	sb.WriteString("    New-Item -ItemType Directory -Path $parentDir -Force | Out-Null\n")
	sb.WriteString("  }\n")
	sb.WriteString("  $fsi = New-Object -ComObject IMAPI2FS.MsftFileSystemImage\n")
	sb.WriteString("  $fsi.FileSystemsToCreate = 3\n") // FsiFileSystemISO9660 (1) + FsiFileSystemJoliet (2)
	sb.WriteString(fmt.Sprintf("  $fsi.VolumeName = %s\n", EscapePSString(opts.VolumeLabel)))
	sb.WriteString("  $fsi.Root.AddTree($tempDir, $false)\n")
	// Compile a small C# helper to copy the COM IStream to a file,
	// since PowerShell can't call IStream::Read directly on the COM object.
	sb.WriteString("  if (-not ([System.Management.Automation.PSTypeName]'ISOStreamWriter').Type) {\n")
	sb.WriteString("    Add-Type -TypeDefinition @'\n")
	sb.WriteString("using System;\n")
	sb.WriteString("using System.IO;\n")
	sb.WriteString("using System.Runtime.InteropServices;\n")
	sb.WriteString("using System.Runtime.InteropServices.ComTypes;\n")
	sb.WriteString("public class ISOStreamWriter {\n")
	sb.WriteString("    public static void Write(object comStream, string path) {\n")
	sb.WriteString("        IStream stream = (IStream)comStream;\n")
	sb.WriteString("        using (FileStream fs = new FileStream(path, FileMode.Create, FileAccess.Write)) {\n")
	sb.WriteString("            byte[] buf = new byte[65536];\n")
	sb.WriteString("            while (true) {\n")
	sb.WriteString("                IntPtr read = Marshal.AllocCoTaskMem(4);\n")
	sb.WriteString("                try {\n")
	sb.WriteString("                    stream.Read(buf, buf.Length, read);\n")
	sb.WriteString("                    int cb = Marshal.ReadInt32(read);\n")
	sb.WriteString("                    if (cb == 0) break;\n")
	sb.WriteString("                    fs.Write(buf, 0, cb);\n")
	sb.WriteString("                } finally { Marshal.FreeCoTaskMem(read); }\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n")
	sb.WriteString("    }\n")
	sb.WriteString("}\n")
	sb.WriteString("'@ -ErrorAction Stop\n")
	sb.WriteString("  }\n")
	sb.WriteString("  $result = $fsi.CreateResultImage()\n")
	sb.WriteString("  [ISOStreamWriter]::Write($result.ImageStream, $isoPath)\n")
	sb.WriteString("  [System.Runtime.InteropServices.Marshal]::ReleaseComObject($fsi) | Out-Null\n")
	sb.WriteString("  $fi = Get-Item $isoPath\n")
	sb.WriteString("  [PSCustomObject]@{ Path = $fi.FullName; Exists = $true; Size = $fi.Length } | ConvertTo-Json\n")
	sb.WriteString("} finally {\n")
	sb.WriteString("  Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue\n")
	sb.WriteString("}\n")

	return sb.String()
}

func buildGetISOCommand(path string) string {
	return fmt.Sprintf(
		"if (Test-Path %[1]s) { $fi = Get-Item %[1]s; [PSCustomObject]@{ Path = $fi.FullName; Exists = $true; Size = $fi.Length } | ConvertTo-Json } else { [PSCustomObject]@{ Path = %[1]s; Exists = $false; Size = 0 } | ConvertTo-Json }",
		EscapePSString(path),
	)
}

func (c *WinRMClient) CreateISO(ctx context.Context, opts ISOOptions) (*ISOInfo, error) {
	var info ISOInfo
	stdinData := buildCreateISOStdinData(opts.Files)
	err := c.ps.RunJSONWithInput(ctx, buildCreateISOScript(opts), stdinData, &info)
	if err != nil {
		return nil, fmt.Errorf("create ISO %q: %w", opts.Path, err)
	}
	return &info, nil
}

func (c *WinRMClient) GetISO(ctx context.Context, path string) (*ISOInfo, error) {
	var info ISOInfo
	err := c.ps.RunJSON(ctx, buildGetISOCommand(path), &info)
	if err != nil {
		return nil, fmt.Errorf("get ISO %q: %w", path, err)
	}
	return &info, nil
}

func (c *WinRMClient) DeleteISO(ctx context.Context, path string) error {
	cmd := fmt.Sprintf("if (Test-Path %[1]s) { Remove-Item -Path %[1]s -Force -ErrorAction Stop }", EscapePSString(path))
	_, _, err := c.ps.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("delete ISO %q: %w", path, err)
	}
	return nil
}
