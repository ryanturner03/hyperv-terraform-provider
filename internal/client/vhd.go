package client

import (
	"context"
	"fmt"
)

func (c *WinRMClient) CreateVHD(ctx context.Context, opts VHDOptions) (*VHD, error) {
	var cmd string
	if opts.Type == "Differencing" {
		cmd = fmt.Sprintf("New-VHD -Path %s -Differencing -ParentPath %s",
			EscapePSString(opts.Path), EscapePSString(opts.ParentPath))
	} else {
		cmd = fmt.Sprintf("New-VHD -Path %s -SizeBytes %d", EscapePSString(opts.Path), opts.SizeBytes)
		if opts.Type == "Fixed" {
			cmd += " -Fixed"
		} else {
			cmd += " -Dynamic"
		}
	}
	if opts.BlockSize > 0 {
		cmd += fmt.Sprintf(" -BlockSizeBytes %d", opts.BlockSize)
	}
	cmd += " -ErrorAction Stop | ConvertTo-Json"

	var vhd VHD
	err := c.ps.RunJSON(ctx, cmd, &vhd)
	if err != nil {
		return nil, fmt.Errorf("create VHD %q: %w", opts.Path, err)
	}
	return &vhd, nil
}

func (c *WinRMClient) GetVHD(ctx context.Context, path string) (*VHD, error) {
	cmd := fmt.Sprintf("Get-VHD -Path %s -ErrorAction Stop | Select-Object Path,Size,VhdType,ParentPath,BlockSize | ConvertTo-Json", EscapePSString(path))
	var vhd VHD
	err := c.ps.RunJSON(ctx, cmd, &vhd)
	if err != nil {
		return nil, fmt.Errorf("get VHD %q: %w", path, err)
	}
	return &vhd, nil
}

func (c *WinRMClient) DeleteVHD(ctx context.Context, path string) error {
	cmd := fmt.Sprintf("if (Test-Path %[1]s) { Remove-Item -Path %[1]s -Force -ErrorAction Stop }", EscapePSString(path))
	_, _, err := c.ps.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("delete VHD %q: %w", path, err)
	}
	return nil
}
