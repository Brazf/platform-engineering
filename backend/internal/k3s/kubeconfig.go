package k3s

import "regexp"

var serverRe = regexp.MustCompile(`server: https://[^:\s]+:6443`)

// rewriteServer troca o endpoint local do k3s.yaml pelo IP da VM.
func rewriteServer(kubeconfig []byte, host string) []byte {
	return serverRe.ReplaceAll(kubeconfig, []byte("server: https://"+host+":6443"))
}
