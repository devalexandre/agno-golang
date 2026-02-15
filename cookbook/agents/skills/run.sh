#!/bin/bash
# Script para rodar o exemplo de skills sem warnings do sqlite3

export GOROOT=/home/devalexandre/.gvm/pkgsets/go1.24.0/global/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.linux-amd64

# Compila (warnings vão para /dev/null)
go build -o /tmp/skills-example ./cookbook/agents/skills/ 2>/dev/null

# Roda o binário (sem warnings de compilação)
/tmp/skills-example
