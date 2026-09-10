package main

import (
	"context"
	"io"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	argossftp "github.com/jhonsferg/argos/integrations/sftp"
)

// ReportPuller fetches report files from an SFTP server. The sample only
// measures the file's size rather than parsing/storing its content - a real
// report-processing pipeline would do more here, but that's business logic
// unrelated to what this sample demonstrates (the SFTP wrapper itself).
type ReportPuller struct {
	client *argossftp.Client
}

// NewReportPuller dials sshAddr and returns a ReportPuller plus a close
// function that tears down both the SFTP and underlying SSH connections.
func NewReportPuller(sshAddr, user, password string) (*ReportPuller, func() error, error) {
	sshClient, err := ssh.Dial("tcp", sshAddr, &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // demo against a local-only container; there's no real host to verify
		Timeout:         10 * time.Second,
	})
	if err != nil {
		return nil, nil, err
	}

	rawClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, nil, err
	}

	closeFn := func() error {
		cErr := rawClient.Close()
		sErr := sshClient.Close()
		if cErr != nil {
			return cErr
		}
		return sErr
	}

	return &ReportPuller{client: argossftp.Wrap(rawClient)}, closeFn, nil
}

// Pull fetches path from the SFTP server and returns its size in bytes.
func (p *ReportPuller) Pull(ctx context.Context, path string) (int64, error) {
	f, err := p.client.OpenContext(ctx, path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	return io.Copy(io.Discard, f)
}
