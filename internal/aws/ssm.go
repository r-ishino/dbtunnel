package aws

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"
)

// PortForwardingSession はSSMポートフォワーディングセッションを表す
type PortForwardingSession struct {
	cmd *exec.Cmd
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

	// コマンドを開始（出力は破棄）
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("SSMセッションの開始に失敗: %w", err)
	}

	// ポートが開くのを待機
	addr := fmt.Sprintf("127.0.0.1:%d", localPort)
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return nil, ctx.Err()
		default:
		}

		// ポートに接続を試みる
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return &PortForwardingSession{cmd: cmd}, nil
		}

		// プロセスが終了していないか確認
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return nil, fmt.Errorf("SSMセッションが予期せず終了しました")
		}

		time.Sleep(500 * time.Millisecond)
	}

	_ = cmd.Process.Kill()
	return nil, fmt.Errorf("SSMセッションの確立がタイムアウトしました（ポート %d が開きませんでした）", localPort)
}

// Stop はポートフォワーディングセッションを停止する
func (s *PortForwardingSession) Stop() error {
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}

// Wait はセッションの終了を待機する
func (s *PortForwardingSession) Wait() error {
	if s.cmd != nil {
		return s.cmd.Wait()
	}
	return nil
}
