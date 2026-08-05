//go:build windows

package cli

// terminatePipeline é um no-op no Windows: process groups POSIX não se
// aplicam, e enviar sinais entre processos de um pipeline não tem
// equivalente direto no console do Windows.
func terminatePipeline() {}
