//go:build !windows

package cli

import "syscall"

// terminatePipeline envia SIGINT ao grupo de processos do próprio kizashi.
// Em "cmd | kizashi stream" (pipe), cmd e kizashi compartilham o mesmo grupo
// (job control do shell); em modo raw o terminal desliga ISIG, então Ctrl-C
// só chega ao kizashi via tcell — sem isto, sair do kizashi (q/Ctrl-C) deixa
// o comando upstream (ex.: um dev server) rodando sozinho, ainda escrevendo
// no terminal, dando a impressão de que o CLI "travou" ao sair. Sinal 0 é o
// próprio grupo do chamador; o próprio kizashi também o recebe, mas de forma
// inofensiva (ainda está escutando via signal.NotifyContext neste ponto).
func terminatePipeline() {
	_ = syscall.Kill(0, syscall.SIGINT)
}
