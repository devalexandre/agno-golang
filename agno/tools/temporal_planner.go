package tools

import (
	"fmt"
	"sort"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// TemporalPlanner planeja e gerencia timelines de tarefas
type TemporalPlanner struct {
	toolkit.Toolkit
	scheduledTasks map[string]ScheduledTask
	executedTasks  []ExecutedTask
	deadlines      map[string]Deadline
}

// ScheduledTask representa uma tarefa agendada
type ScheduledTask struct {
	TaskID       string    `json:"task_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	ScheduledFor time.Time `json:"scheduled_for"`
	Duration     int       `json:"duration_minutes"`
	Priority     int       `json:"priority"` // 1-5, 5 é máxima
	Dependencies []string  `json:"dependencies"`
	Status       string    `json:"status"` // pending, ready, running, completed, cancelled
	Tags         []string  `json:"tags"`
	CreatedAt    time.Time `json:"created_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	Recurrence   string    `json:"recurrence,omitempty"` // "daily", "weekly", "monthly", "none"
}

// ExecutedTask registra execução de tarefa
type ExecutedTask struct {
	TaskID         string    `json:"task_id"`
	Title          string    `json:"title"`
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    time.Time `json:"completed_at"`
	ActualDuration int       `json:"actual_duration_minutes"`
	Status         string    `json:"status"` // success, failed, partial
	Notes          string    `json:"notes"`
	ErrorMessage   string    `json:"error_message,omitempty"`
}

// Deadline representa um deadline crítico
type Deadline struct {
	DeadlineID      string    `json:"deadline_id"`
	Title           string    `json:"title"`
	DueDate         time.Time `json:"due_date"`
	Priority        int       `json:"priority"`
	AssociatedTasks []string  `json:"associated_tasks"`
	Status          string    `json:"status"` // upcoming, active, overdue, completed
}

// ScheduleTaskParams parâmetros para agendar tarefa
type ScheduleTaskParams struct {
	Title        string   `json:"title" description:"Título da tarefa"`
	Description  string   `json:"description" description:"Descrição da tarefa"`
	ScheduledFor string   `json:"scheduled_for" description:"Data/hora (RFC3339)"`
	Duration     int      `json:"duration" description:"Duração em minutos"`
	Priority     int      `json:"priority" description:"Prioridade 1-5"`
	Dependencies []string `json:"dependencies" description:"IDs de tarefas dependentes"`
	Recurrence   string   `json:"recurrence" description:"daily, weekly, monthly, none"`
}

// GetUpcomingTasksParams parâmetros para obter tarefas próximas
type GetUpcomingTasksParams struct {
	Hours    int    `json:"hours" description:"Próximas N horas"`
	Priority int    `json:"priority" description:"Filtrar por prioridade mínima"`
	Status   string `json:"status" description:"Filtrar por status"`
}

// ExecuteTaskParams parâmetros para marcar execução
type ExecuteTaskParams struct {
	TaskID   string `json:"task_id" description:"ID da tarefa"`
	Status   string `json:"status" description:"success, failed, partial"`
	Notes    string `json:"notes" description:"Notas sobre execução"`
	ErrorMsg string `json:"error_message" description:"Mensagem de erro se falhou"`
}

// TaskResult resultado de operação com tarefas
type TaskResult struct {
	Success   bool            `json:"success"`
	TaskID    string          `json:"task_id,omitempty"`
	Message   string          `json:"message"`
	Task      ScheduledTask   `json:"task,omitempty"`
	Tasks     []ScheduledTask `json:"tasks,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewTemporalPlanner cria novo planner
func NewTemporalPlanner() *TemporalPlanner {
	p := &TemporalPlanner{
		scheduledTasks: make(map[string]ScheduledTask),
		executedTasks:  make([]ExecutedTask, 0),
		deadlines:      make(map[string]Deadline),
	}
	p.Toolkit = toolkit.NewToolkit()

	p.Toolkit.Register(
		"ScheduleTask",
		"Agendar nova tarefa com data e prioridade",
		p,
		p.ScheduleTask,
		ScheduleTaskParams{},
	)

	p.Toolkit.Register(
		"GetUpcomingTasks",
		"Obter tarefas agendadas para próximas horas",
		p,
		p.GetUpcomingTasks,
		GetUpcomingTasksParams{},
	)

	p.Toolkit.Register(
		"ExecuteTask",
		"Registrar execução de uma tarefa",
		p,
		p.ExecuteTask,
		ExecuteTaskParams{},
	)

	p.Toolkit.Register(
		"GetTaskTimeline",
		"Obter timeline de tarefas (visual)",
		p,
		p.GetTaskTimeline,
		GetTimelineParams{},
	)

	p.Toolkit.Register(
		"CreateDeadline",
		"Criar deadline crítico",
		p,
		p.CreateDeadline,
		CreateDeadlineParams{},
	)

	p.Toolkit.Register(
		"GetDeadlineStatus",
		"Obter status de deadlines",
		p,
		p.GetDeadlineStatus,
		GetDeadlineParams{},
	)

	return p
}

// ScheduleTask agenda nova tarefa
func (p *TemporalPlanner) ScheduleTask(params ScheduleTaskParams) (interface{}, error) {
	if params.Title == "" {
		return TaskResult{Success: false}, fmt.Errorf("título obrigatório")
	}

	scheduledTime, err := time.Parse(time.RFC3339, params.ScheduledFor)
	if err != nil {
		return TaskResult{Success: false}, fmt.Errorf("formato de data inválido: %v", err)
	}

	// Gerar ID
	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())

	task := ScheduledTask{
		TaskID:       taskID,
		Title:        params.Title,
		Description:  params.Description,
		ScheduledFor: scheduledTime,
		Duration:     params.Duration,
		Priority:     params.Priority,
		Dependencies: params.Dependencies,
		Status:       "pending",
		Tags:         make([]string, 0),
		CreatedAt:    time.Now(),
		Recurrence:   params.Recurrence,
	}

	// Validar prioridade
	if task.Priority < 1 || task.Priority > 5 {
		task.Priority = 3
	}

	p.scheduledTasks[taskID] = task

	return TaskResult{
		Success:   true,
		TaskID:    taskID,
		Message:   fmt.Sprintf("Tarefa '%s' agendada para %s", params.Title, scheduledTime.Format("02/01 15:04")),
		Task:      task,
		Timestamp: time.Now(),
	}, nil
}

// GetUpcomingTasks retorna tarefas próximas
func (p *TemporalPlanner) GetUpcomingTasks(params GetUpcomingTasksParams) (interface{}, error) {
	if params.Hours <= 0 {
		params.Hours = 24
	}

	now := time.Now()
	futureTime := now.Add(time.Duration(params.Hours) * time.Hour)

	upcoming := make([]ScheduledTask, 0)

	for _, task := range p.scheduledTasks {
		// Filtrar por tempo
		if task.ScheduledFor.After(now) && task.ScheduledFor.Before(futureTime) {
			// Filtrar por prioridade
			if params.Priority > 0 && task.Priority < params.Priority {
				continue
			}

			// Filtrar por status
			if params.Status != "" && task.Status != params.Status {
				continue
			}

			upcoming = append(upcoming, task)
		}
	}

	// Ordenar por tempo (mais próximo primeiro)
	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].ScheduledFor.Before(upcoming[j].ScheduledFor)
	})

	return TaskResult{
		Success:   true,
		Tasks:     upcoming,
		Message:   fmt.Sprintf("%d tarefas próximas nos próximos %d horas", len(upcoming), params.Hours),
		Timestamp: time.Now(),
	}, nil
}

// ExecuteTask registra execução de tarefa
func (p *TemporalPlanner) ExecuteTask(params ExecuteTaskParams) (interface{}, error) {
	if params.TaskID == "" {
		return TaskResult{Success: false}, fmt.Errorf("task_id obrigatório")
	}

	task, exists := p.scheduledTasks[params.TaskID]
	if !exists {
		return TaskResult{Success: false}, fmt.Errorf("tarefa não encontrada")
	}

	completedAt := time.Now()
	duration := int(completedAt.Sub(task.ScheduledFor).Minutes())

	executed := ExecutedTask{
		TaskID:         params.TaskID,
		Title:          task.Title,
		StartedAt:      task.ScheduledFor,
		CompletedAt:    completedAt,
		ActualDuration: duration,
		Status:         params.Status,
		Notes:          params.Notes,
		ErrorMessage:   params.ErrorMsg,
	}

	p.executedTasks = append(p.executedTasks, executed)

	// Atualizar status da tarefa original
	task.Status = "completed"
	task.CompletedAt = completedAt
	p.scheduledTasks[params.TaskID] = task

	// Se recorrente, agendar próxima
	if task.Recurrence != "none" && task.Recurrence != "" {
		p.scheduleNextRecurrence(task)
	}

	return TaskResult{
		Success:   true,
		TaskID:    params.TaskID,
		Message:   fmt.Sprintf("Tarefa completada com status: %s", params.Status),
		Timestamp: time.Now(),
	}, nil
}

// GetTaskTimeline retorna timeline visual
func (p *TemporalPlanner) GetTaskTimeline(params GetTimelineParams) (interface{}, error) {
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)

	todayTasks := make([]ScheduledTask, 0)
	tomorrowTasks := make([]ScheduledTask, 0)

	for _, task := range p.scheduledTasks {
		if task.Status == "completed" || task.Status == "cancelled" {
			continue
		}

		if task.ScheduledFor.YearDay() == now.YearDay() {
			todayTasks = append(todayTasks, task)
		} else if task.ScheduledFor.YearDay() == tomorrow.YearDay() {
			tomorrowTasks = append(tomorrowTasks, task)
		}
	}

	// Ordenar por hora
	sort.Slice(todayTasks, func(i, j int) bool {
		return todayTasks[i].ScheduledFor.Before(todayTasks[j].ScheduledFor)
	})

	sort.Slice(tomorrowTasks, func(i, j int) bool {
		return tomorrowTasks[i].ScheduledFor.Before(tomorrowTasks[j].ScheduledFor)
	})

	return map[string]interface{}{
		"timestamp":      time.Now(),
		"today_count":    len(todayTasks),
		"today_tasks":    todayTasks,
		"tomorrow_count": len(tomorrowTasks),
		"tomorrow_tasks": tomorrowTasks,
		"total_pending":  len(p.scheduledTasks),
	}, nil
}

// CreateDeadline cria deadline crítico
func (p *TemporalPlanner) CreateDeadline(params CreateDeadlineParams) (interface{}, error) {
	if params.Title == "" {
		return TaskResult{Success: false}, fmt.Errorf("título obrigatório")
	}

	dueDate, err := time.Parse(time.RFC3339, params.DueDate)
	if err != nil {
		return TaskResult{Success: false}, fmt.Errorf("data inválida")
	}

	deadlineID := fmt.Sprintf("deadline_%d", time.Now().UnixNano())

	deadline := Deadline{
		DeadlineID:      deadlineID,
		Title:           params.Title,
		DueDate:         dueDate,
		Priority:        params.Priority,
		AssociatedTasks: params.AssociatedTasks,
		Status:          "upcoming",
	}

	p.deadlines[deadlineID] = deadline

	return map[string]interface{}{
		"success":     true,
		"deadline_id": deadlineID,
		"title":       params.Title,
		"due_date":    dueDate.Format("02/01/2006 15:04"),
		"timestamp":   time.Now(),
	}, nil
}

// GetDeadlineStatus obtém status de deadlines
func (p *TemporalPlanner) GetDeadlineStatus(params GetDeadlineParams) (interface{}, error) {
	now := time.Now()
	active := make([]Deadline, 0)
	overdue := make([]Deadline, 0)

	for _, deadline := range p.deadlines {
		if deadline.DueDate.Before(now) {
			deadline.Status = "overdue"
			overdue = append(overdue, deadline)
		} else if deadline.DueDate.Before(now.AddDate(0, 0, 1)) {
			deadline.Status = "active"
			active = append(active, deadline)
		}
	}

	return map[string]interface{}{
		"active_count":   len(active),
		"active":         active,
		"overdue_count":  len(overdue),
		"overdue":        overdue,
		"total_deadline": len(p.deadlines),
		"timestamp":      time.Now(),
	}, nil
}

// Helper functions

func (p *TemporalPlanner) scheduleNextRecurrence(task ScheduledTask) {
	var nextTime time.Time

	switch task.Recurrence {
	case "daily":
		nextTime = task.ScheduledFor.AddDate(0, 0, 1)
	case "weekly":
		nextTime = task.ScheduledFor.AddDate(0, 0, 7)
	case "monthly":
		nextTime = task.ScheduledFor.AddDate(0, 1, 0)
	default:
		return
	}

	newTask := task
	newTask.TaskID = fmt.Sprintf("task_%d", time.Now().UnixNano())
	newTask.ScheduledFor = nextTime
	newTask.Status = "pending"
	newTask.CreatedAt = time.Now()

	p.scheduledTasks[newTask.TaskID] = newTask
}

// GetTimelineParams parâmetros para obter timeline
type GetTimelineParams struct {
	Days int `json:"days" description:"Número de dias a mostrar"`
}

// CreateDeadlineParams parâmetros para criar deadline
type CreateDeadlineParams struct {
	Title           string   `json:"title" description:"Título do deadline"`
	DueDate         string   `json:"due_date" description:"Data de vencimento (RFC3339)"`
	Priority        int      `json:"priority" description:"Prioridade 1-5"`
	AssociatedTasks []string `json:"associated_tasks" description:"IDs de tarefas associadas"`
}

// GetDeadlineParams parâmetros para obter deadlines
type GetDeadlineParams struct {
	OnlyOverdue bool `json:"only_overdue" description:"Mostrar apenas vencidos"`
}
