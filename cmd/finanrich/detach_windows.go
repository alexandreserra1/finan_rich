package main

import "os/exec"

// detach: no Windows, fechar o console não fecha processos já iniciados.
func detach(*exec.Cmd) {}
