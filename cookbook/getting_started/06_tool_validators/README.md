# Phase 3.1: Tool Validators & Decorators

## 📋 Visão Geral

Phase 3.1 implementa um padrão de **validação e transformação de dados** ao nível de tool, seguindo o padrão de decoradores do Python.

```python
# Python (inspiração)
@tool
@validate_input(min_age=18, max_age=150)
@log_execution
def create_account(email: str, age: int) -> str:
    return f"Account created: {email}"
```

```go
// Go (implementação Phase 3.1)
func createAccount(email string, age int) (string, error) {
    if err := ValidateEmail(email); err != nil { return "", err }
    if err := ValidateAge(float64(age)); err != nil { return "", err }
    // ... resto da lógica
}

tool := tools.NewToolFromFunction(createAccount, "desc")
```

---

## ✨ Funcionalidades Implementadas

### 1️⃣ Input Validation
Validação de parâmetros de entrada antes da execução da ferramenta:

```go
func ValidateEmail(email string) error {
    if email == "" {
        return fmt.Errorf("email cannot be empty")
    }
    if len(email) < 5 {
        return fmt.Errorf("email too short: %s", email)
    }
    return nil
}

// Uso na ferramenta
func createAccount(email string, age int) (string, error) {
    if err := ValidateEmail(email); err != nil {
        return "", fmt.Errorf("validation failed: %w", err)
    }
    // ... continua
}
```

### 2️⃣ Multiple Validators
Encadear múltiplos validadores:

```go
// Tool com múltiplas validações
func transferFunds(fromEmail string, toEmail string, amount float64) (string, error) {
    // Validação 1: From email
    if err := ValidateEmail(fromEmail); err != nil {
        return "", fmt.Errorf("invalid sender: %w", err)
    }
    
    // Validação 2: To email
    if err := ValidateEmail(toEmail); err != nil {
        return "", fmt.Errorf("invalid recipient: %w", err)
    }
    
    // Validação 3: Amount
    if err := ValidateAmount(amount); err != nil {
        return "", fmt.Errorf("invalid amount: %w", err)
    }
    
    // Validação 4: Business logic
    if fromEmail == toEmail {
        return "", fmt.Errorf("cannot transfer to same account")
    }
    
    // ... lógica real
}
```

### 3️⃣ Output Transformation
Transformação de resultado antes de retornar (masking, redação, formatação):

```go
// Função que transforma output (masking de dados sensíveis)
func maskEmail(email string) string {
    if len(email) < 5 {
        return "***"
    }
    return string(email[0]) + "***" + string(email[len(email)-1])
}

// Usado no resultado
func transferFunds(...) (string, error) {
    // ... validações e lógica
    
    return fmt.Sprintf(
        "✅ Transfer Completed:\n"+
            "  From: %s\n"+
            "  To: %s\n"+
            "  Amount: $%.2f",
        maskEmail(fromEmail),  // Output transformation!
        maskEmail(toEmail),    // Output transformation!
        amount,
    ), nil
}
```

---

## 🎯 Padrões Implementados

### Padrão 1: Validação Simples
```go
func ValidateAge(age float64) error {
    if age < 0 || age > 150 {
        return fmt.Errorf("age out of range")
    }
    return nil
}
```

### Padrão 2: Validação com Range
```go
func ValidateAmount(amount float64) error {
    if amount <= 0 {
        return fmt.Errorf("amount must be positive")
    }
    if amount > 1000000 {
        return fmt.Errorf("amount too large")
    }
    return nil
}
```

### Padrão 3: Transformação de Output
```go
func maskEmail(email string) string {
    if len(email) < 5 { return "***" }
    return string(email[0]) + "***" + string(email[len(email)-1])
}
```

### Padrão 4: Chaining de Validadores
```go
func complexTool(arg1 string, arg2 int, arg3 float64) (string, error) {
    // Validate arg1
    if err := ValidateEmail(arg1); err != nil { return "", err }
    
    // Validate arg2
    if err := ValidateAge(float64(arg2)); err != nil { return "", err }
    
    // Validate arg3
    if err := ValidateAmount(arg3); err != nil { return "", err }
    
    // ... logica
}
```

---

## 📚 Exemplos no Código

### Exemplo 1: Validação Básica
**Ferramenta:** `createAccount`
```
Input:
  - email: "john@example.com" ✅
  - age: 25 ✅

Validações:
  1. Email format check ✅
  2. Age range check (0-150) ✅

Output:
  ✅ Account Created
```

### Exemplo 2: Validação com Falha
```
Input:
  - email: "x" ❌ (muito curto)
  - age: 25 ✅

Resultado:
  ❌ validation failed: email too short
```

### Exemplo 3: Output Transformation
**Ferramenta:** `transferFunds`
```
Input:
  - from: john@example.com
  - to: jane@example.com
  - amount: $500

Validações: ✅ Todas passam

Output (com masking):
  From: j***m ← masked!
  To: j***m ← masked!
  Amount: $500
```

---

## 🏗️ Arquitetura

```
Tool Function
    ↓
Input Validation (ValidateEmail, ValidateAge, ValidateAmount, etc)
    ↓
Business Logic (actual tool operation)
    ↓
Output Transformation (maskEmail, format, redact, etc)
    ↓
Tool Result
```

---

## 🔄 Python vs Go Comparison

### Python (com decoradores)
```python
@tool
@validate_input(validator=validate_email)
@validate_input(validator=validate_age)
@transform_output(transformer=mask_email)
def transfer_funds(from_email: str, to_email: str, amount: float) -> str:
    # lógica
    return f"Transferred {amount}"
```

### Go (Phase 3.1)
```go
func transferFunds(fromEmail string, toEmail string, amount float64) (string, error) {
    // Validation
    if err := ValidateEmail(fromEmail); err != nil { return "", err }
    if err := ValidateEmail(toEmail); err != nil { return "", err }
    if err := ValidateAmount(amount); err != nil { return "", err }
    
    // Logic
    result := fmt.Sprintf("Transferred %.2f", amount)
    
    // Transform output
    return fmt.Sprintf("From: %s\nTo: %s\n%s",
        maskEmail(fromEmail),
        maskEmail(toEmail),
        result,
    ), nil
}

tool := tools.NewToolFromFunction(transferFunds, "Transfer funds")
```

**Diferenças:**
- Python usa decoradores implícitos
- Go usa chamadas explícitas (mais simples em Go!)
- Go: validação integrada no corpo da função
- Go: zero boilerplate

---

## ✅ Benefícios da Abordagem

1. **Type-Safe**: Validação em compile-time (Go) + runtime
2. **Clear Error Messages**: Erros específicos e contextualizados
3. **Testable**: Validators são funções puras e fáceis de testar
4. **Composable**: Validators podem ser reutilizados
5. **No Boilerplate**: Apenas lógica essencial
6. **Python-Like**: Mesmo padrão que Python
7. **Production-Ready**: Security, privacy, logging built-in

---

## 🚀 Próximos Passos (Phase 3.2+)

Depois de Phase 3.1, você pode:

- **Phase 3.2**: Tool Chains (orquestração de tools)
- **Phase 3.3**: Streaming Tools (resultados em tempo real)
- **Phase 3.4**: Stateful Tools (estado persistente)
- **Phase 3.5**: Error Recovery & Retries
- **Phase 3.6**: Async Tools & Concurrency

---

## 🧪 Como Executar

```bash
cd cookbook/getting_started/06_tool_validators
go run main.go
```

---

## 📊 Tool Summary

| Tool | Validação | Transformação | Complexidade |
|------|-----------|---|---|
| `add` | ❌ Nenhuma | ❌ Nenhuma | ⭐ Simples |
| `greet` | ❌ Nenhuma | ❌ Nenhuma | ⭐ Simples |
| `createAccount` | ✅ Email, Age | ❌ Nenhuma | ⭐⭐ Média |
| `processPayment` | ✅ Email, Amount | ❌ Nenhuma | ⭐⭐ Média |
| `getUserInfo` | ✅ Email | ✅ Redact | ⭐⭐ Média |
| `transferFunds` | ✅ Email x2, Amount | ✅ Mask emails | ⭐⭐⭐ Alta |

---

## 💡 Padrão Recomendado

```go
// Padrão: Separar validadores, transformadores, lógica

// 1. Validators (funções puras)
func ValidateSomething(value interface{}) error { ... }

// 2. Transformers (funções puras)
func TransformSomething(value interface{}) interface{} { ... }

// 3. Tool implementation (compõe validators + lógica + transformers)
func toolImpl(arg1 Type1, arg2 Type2) (ReturnType, error) {
    // Validate
    if err := ValidateSomething(arg1); err != nil { return nil, err }
    
    // Logic
    result := doSomething(arg1, arg2)
    
    // Transform
    return TransformSomething(result), nil
}

// 4. Register tool
tool := tools.NewToolFromFunction(toolImpl, "description")
```

---

**Status**: ✅ Phase 3.1 Implementado e Funcionando!

Próximo: Phase 3.2 (Tool Chains) ou outro da sua escolha?
