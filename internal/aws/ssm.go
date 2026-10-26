package aws

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"time"
)

// PortForwardingSession はSSMポートフォワーディングセッションを表す
type PortForwardingSession struct {
	cmd    *exec.Cmd
	stderr *bytes.Buffer
	done   chan struct{}
	err    error
}

// StartPortForwarding はSSMポートフォワーディングを開始する
func StartPortForwarding(
	ctx context.Context,
	profile string,
	region string,
	ssmTarget string,
	remoteHost string,
	remotePort int,
	localPort int,
) (*PortForwardingSession, error) {
	// パラメータをJSON形式で構築
	params := fmt.Sprintf(`{"host":["%s"],"portNumber":["%d"],"localPortNumber":["%d"]}`,
		remoteHost, remotePort, localPort)

	// aws ssm start-session コマンドを構築
	cmd := exec.CommandContext(ctx, "aws", "ssm", "start-session",
		"--target", ssmTarget,
		"--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
		"--parameters", params,
		"--profile", profile,
		"--region", region,
	)

	session := &PortForwardingSession{
		cmd:    cmd,
		stderr: &bytes.Buffer{},
		done:   make(chan struct{}),
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = session.stderr

	// コマンドを開始
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("SSMセッションの開始に失敗: %w", err)
	}

	go func() {
		session.err = cmd.Wait()
		close(session.done)
	}()

	// ポートが開くのを待機
	addr := fmt.Sprintf("127.0.0.1:%d", localPort)
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return nil, ctx.Err()
		case <-session.done:
			return nil, fmt.Errorf("SSMセッションが予期せず終了しました: %w", session.Err())
		default:
		}

		// ポートに接続を試みる
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return session, nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	_ = cmd.Process.Kill()
	<-session.done
	return nil, fmt.Errorf("SSMセッションの確立がタイムアウトしました（ポート %d が開きませんでした）: %w", localPort, session.Err())
}

// Stop はポートフォワーディングセッションを停止する
func (s *PortForwardingSession) Stop() error {
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}

// Done はセッションのプロセスが終了すると閉じるチャネルを返す
func (s *PortForwardingSession) Done() <-chan struct{} {
	return s.done
}

// Err は終了したセッションのエラーを、aws CLIが標準エラーに出した内容を添えて返す。Doneが閉じる前はnilを返す
func (s *PortForwardingSession) Err() error {
	select {
	case <-s.done:
	default:
		return nil
	}

	err := s.err
	if err == nil {
		err = errors.New("exit status 0")
	}
	output := strings.TrimSpace(s.stderr.String())
	if output == "" {
		return err
	}
	return fmt.Errorf("%w\n%s", err, output)
}
