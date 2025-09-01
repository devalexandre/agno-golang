#!/bin/bash

echo "🚀 Otimizando Ollama para Performance..."

# Configurações de ambiente para otimização
export OLLAMA_NUM_PARALLEL=1
export OLLAMA_MAX_LOADED_MODELS=1
export OLLAMA_FLASH_ATTENTION=1
export CUDA_VISIBLE_DEVICES=0

# Reiniciar Ollama com configurações otimizadas
echo "Parando Ollama..."
pkill ollama

echo "Iniciando Ollama otimizado..."
OLLAMA_NUM_PARALLEL=1 OLLAMA_MAX_LOADED_MODELS=1 ollama serve &

sleep 3

echo "✅ Ollama otimizado iniciado!"
echo ""
echo "📋 Configurações aplicadas:"
echo "  - OLLAMA_NUM_PARALLEL=1 (reduz uso de RAM)"
echo "  - OLLAMA_MAX_LOADED_MODELS=1 (carrega apenas 1 modelo)"
echo "  - OLLAMA_FLASH_ATTENTION=1 (ativa otimização de atenção)"
echo ""
echo "💡 Para tornar permanente, adicione ao ~/.bashrc:"
echo "export OLLAMA_NUM_PARALLEL=1"
echo "export OLLAMA_MAX_LOADED_MODELS=1"
echo "export OLLAMA_FLASH_ATTENTION=1"
