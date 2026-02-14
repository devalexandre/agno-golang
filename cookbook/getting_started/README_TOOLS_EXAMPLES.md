# Tools: De Simples até Avançado

## Progressão de Exemplos

Este diretório contém exemplos progressivos de como usar ferramentas no Agno:

### 📌 04_simple_tools - Básico

**Foco:** Entender o básico de ferramentas com tipos simples

**Tópicos:**
- ✅ Criar ferramentas de funções simples
- ✅ Tipos primitivos (int, string)
- ✅ Integração com Agent
- ✅ Exemplo Python-like

**Compile e execute:**
```bash
cd 04_simple_tools
go build
./04_simple_tools
```

**Ferramentas no exemplo:**
- `add(int, int) int` - Soma dois números
- `multiply(int, int) int` - Multiplica dois números
- `greet(string) string` - Cumprimenta alguém

**Saída esperada:**
```
=== Simple Tools Example (Python-like API) ===

Example 1: Math question
Agent: Using add and multiply tools...
Result: 5 + 3 = 8, 8 * 2 = 16

Example 2: Greeting
Agent: Using greet tool...
Result: Hello, Alice!

Example 3: Combined question
Agent: Using multiply and greet tools...
Result: 10 * 5 = 50, greeting Bob...
```

---

### 🚀 05_advanced_struct_tools - Avançado

**Foco:** Trabalhar com structs complexas e aninhadas

**Tópicos:**
- ✅ Structs como parâmetros
- ✅ Structs aninhadas (nested)
- ✅ Arrays de structs
- ✅ Tipos de retorno complexos
- ✅ Schema geração automática
- ✅ Casos de uso do mundo real

**Compile e execute:**
```bash
cd 05_advanced_struct_tools
go build
./05_advanced_struct_tools
```

**Ferramentas no exemplo:**

1. **Simples** (para comparação)
   - `add(int, int) int`
   - `greet(string) string`

2. **Com Struct**
   - `createUserProfile(User) string`
   ```go
   type User struct {
       ID       int      `json:"id"`
       Name     string   `json:"name"`
       Email    string   `json:"email"`
       Age      int      `json:"age"`
       Skills   []string `json:"skills"`
       Active   bool     `json:"active"`
       JoinDate string   `json:"join_date"`
   }
   ```

3. **Com Struct Aninhada**
   - `searchWeather(WeatherQuery) string`
   ```go
   type WeatherQuery struct {
       Location  Location `json:"location"`  // Nested!
       DateRange string   `json:"date_range"`
       Metrics   []string `json:"metrics"`
   }
   
   type Location struct {
       Latitude  float64 `json:"latitude"`
       Longitude float64 `json:"longitude"`
       City      string  `json:"city"`
       Country   string  `json:"country"`
   }
   ```

4. **Com Return Complexo**
   - `bookHotel(BookingRequest) BookingResponse`
   ```go
   type BookingRequest struct {
       CustomerName    string   `json:"customer_name"`
       Email           string   `json:"email"`
       CheckIn         string   `json:"check_in"`
       CheckOut        string   `json:"check_out"`
       RoomType        string   `json:"room_type"`
       Guests          int      `json:"guests"`
       SpecialRequests []string `json:"special_requests"`
   }
   
   type BookingResponse struct {
       BookingID    string  `json:"booking_id"`
       Status       string  `json:"status"`
       ConfirmEmail string  `json:"confirm_email"`
       TotalPrice   float64 `json:"total_price"`
   }
   ```

5. **Com Array de Structs**
   - `processMultipleUsers([]User) string`

**Saída esperada:**
```
================================================================================
ADVANCED EXAMPLE: Tools with Complex Struct Parameters
================================================================================

📌 Section 1: Simple Tools (baseline)
✓ Created simple tools...

📌 Section 2: Tools with Struct Parameters
✓ Created user profile tool...

📌 Section 3: Tools with Nested Structs
✓ Created weather search tool...

📌 Section 4: Tools with Complex Return Types
✓ Created hotel booking tool...

📌 Section 5: Tools with Array Parameters
✓ Created multi-user processing tool...

📌 Section 6: Agent Integration
✓ Created agent with 6 tools

📌 Section 7: Example Tool Executions

Example 1: Simple math tool
...

Example 2: User profile creation
...

Example 3: Weather search with nested location
...

Example 4: Hotel booking
...

✅ Advanced Tools Example Complete!

Key Takeaways:
✓ Structs work seamlessly as tool parameters
✓ Nested structs are fully supported
✓ Complex return types are handled automatically
✓ Arrays of structs work as parameters
✓ Type conversion happens automatically
✓ Same simple API as Python - no boilerplate!
```

---

## Comparação: 04 vs 05

| Aspecto | 04_simple_tools | 05_advanced_struct_tools |
|---------|-----------------|-------------------------|
| **Tipos de Parâmetros** | Primitivos (int, string) | Structs (simples e aninhadas) |
| **Complexidade** | ⭐ Básica | ⭐⭐⭐⭐⭐ Avançada |
| **Ferramentas** | 3 ferramentas | 6 ferramentas |
| **Exemplos de Uso** | Matemática, saudação | Perfil, booking, clima |
| **Type Safety** | Básica | Completa |
| **Casos de Uso** | Aprender fundamentos | Aplicações reais |
| **Tempo para entender** | 5 minutos | 15 minutos |

---

## Recommended Learning Progression

### Nível 1: Iniciante
1. Leia `04_simple_tools/main.go`
2. Execute e veja funcionando
3. Entenda: função → tool → agent

### Nível 2: Intermediário
1. Leia `PHASE_2_TOOL_SYSTEM.md` no root
2. Entenda como schema é gerado
3. Veja API Python-like em Go

### Nível 3: Avançado
1. Estude `05_advanced_struct_tools/main.go`
2. Leia `ADVANCED_STRUCT_TOOLS_GUIDE.md` no root
3. Veja como structs funcionam com reflection
4. Entenda mapeamento JSON automático

### Nível 4: Expert
1. Explore `agno/tools/tool.go` (implementação)
2. Entenda reflection e type conversion
3. Crie seus próprios tipos complexos
4. Otimize para seu caso de uso

---

## Cheat Sheet: Como Usar

### Tipos Simples ✅
```go
func add(a int, b int) (int, error)
func search(query string) (string, error)
```

### Structs ✅
```go
type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

func createProfile(user User) (string, error)
```

### Structs Aninhadas ✅
```go
type Location struct {
    Lat float64 `json:"lat"`
    Lon float64 `json:"lon"`
}

type Query struct {
    Location Location `json:"location"`
    Query    string   `json:"query"`
}

func search(q Query) (string, error)
```

### Arrays ✅
```go
func processUsers(users []User) (string, error)
```

### Retorno Complexo ✅
```go
type Result struct {
    ID    string  `json:"id"`
    Price float64 `json:"price"`
    Status string  `json:"status"`
}

func book(request Request) (Result, error)
```

---

## Recursos Adicionais

### Documentação Principal
- `PHASE_2_TOOL_SYSTEM.md` - Visão geral do sistema
- `ADVANCED_STRUCT_TOOLS_GUIDE.md` - Guia completo de structs

### Código
- `agno/tools/tool.go` - Implementação
- `agno/tools/contracts.go` - Tipos de contrato

### Exemplos
- `04_simple_tools/main.go` - Tipos primitivos
- `05_advanced_struct_tools/main.go` - Structs complexas

---

## FAQ

### P: Posso usar qualquer tipo como parâmetro?
**R:** Sim! Qualquer tipo Go que pode ser serializado para JSON.

### P: Preciso adicionar JSON tags?
**R:** Altamente recomendado para clareza e consistência.

### P: Qual a profundidade máxima de aninhamento?
**R:** Nenhuma limitação técnica, mas mantenha simples para clareza.

### P: Como o Agent sabe quais valores usar?
**R:** O schema JSON descreve tudo, e o modelo LLM interpreta corretamente.

### P: Posso retornar erros?
**R:** Sim! Use `error` como último retorno (padrão Go).

### P: Funciona com tipos customizados?
**R:** Sim! `type MyID string` etc funcionam.

---

## Próximos Passos

Após dominar estes exemplos:

1. **Exemplo 06** - Ferramentas com validação complexa
2. **Exemplo 07** - Ferramentas com estado
3. **Exemplo 08** - Ferramentas com context
4. **Exemplo 09** - Múltiplas ferramentas coordenadas
5. **Exemplo 10** - Caso de uso: Chatbot de E-commerce

---

## Comparação Python ↔ Go

### Python - 04_simple_tools equivalente
```python
from agno.tools import tool

@tool
def add(a: int, b: int) -> int:
    return a + b

agent = Agent(tools=[add], model=model)
```

### Go - Equivalente
```go
func add(a int, b int) (int, error) {
    return a + b, nil
}

tool := tools.NewToolFromFunction(add, "Add two numbers")
agent := agent.NewAgent(agent.AgentConfig{
    Tools: []toolkit.Tool{tool},
    Model: model,
})
```

---

## Status

- ✅ `04_simple_tools` - Funcional e testado
- ✅ `05_advanced_struct_tools` - Funcional e testado
- ⏳ Mais exemplos em desenvolvimento

---

**Comece pelo `04_simple_tools` e progresse para o `05_advanced_struct_tools`!**
