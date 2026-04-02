package task

type TaskStore interface {
	CreateTask(t *Task) error
	GetTask(id string) (*Task, error)
	ListTasks(status string, project string) ([]*Task, error)
	UpdateTask(t *Task) error
	DeleteTask(id string) error
	CreateContextEntry(e *ContextEntry) error
	ListContextEntries(taskID string) ([]*ContextEntry, error)
}

type Service struct {
	store TaskStore
}

func NewService(store TaskStore) *Service {
	return &Service{store: store}
}

func (s *Service) Create(t *Task) error {
	return s.store.CreateTask(t)
}

func (s *Service) Get(id string) (*Task, error) {
	return s.store.GetTask(id)
}

func (s *Service) List(status string, project string) ([]*Task, error) {
	return s.store.ListTasks(status, project)
}

func (s *Service) Update(t *Task) error {
	return s.store.UpdateTask(t)
}

func (s *Service) Delete(id string) error {
	return s.store.DeleteTask(id)
}

func (s *Service) AddContext(entry *ContextEntry) error {
	return s.store.CreateContextEntry(entry)
}

func (s *Service) ListContext(taskID string) ([]*ContextEntry, error) {
	return s.store.ListContextEntries(taskID)
}
