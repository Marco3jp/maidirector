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

type StoryStore interface {
	CreateStory(st *Story) error
	GetStory(id string) (*Story, error)
	ListStories(status string) ([]*Story, error)
	UpdateStory(st *Story) error
	DeleteStory(id string) error
	CreateStoryLayer(l *StoryLayer) error
	GetStoryLayer(id string) (*StoryLayer, error)
	ListStoryLayers(storyID string) ([]*StoryLayer, error)
	UpdateStoryLayer(l *StoryLayer) error
	DeleteStoryLayer(id string) error
}

type StoryService struct {
	store StoryStore
}

func NewStoryService(store StoryStore) *StoryService {
	return &StoryService{store: store}
}

func (s *StoryService) Create(st *Story) error {
	return s.store.CreateStory(st)
}

func (s *StoryService) Get(id string) (*Story, error) {
	return s.store.GetStory(id)
}

func (s *StoryService) List(status string) ([]*Story, error) {
	return s.store.ListStories(status)
}

func (s *StoryService) Update(st *Story) error {
	return s.store.UpdateStory(st)
}

func (s *StoryService) Delete(id string) error {
	return s.store.DeleteStory(id)
}

func (s *StoryService) AddLayer(l *StoryLayer) error {
	return s.store.CreateStoryLayer(l)
}

func (s *StoryService) GetLayer(id string) (*StoryLayer, error) {
	return s.store.GetStoryLayer(id)
}

func (s *StoryService) ListLayers(storyID string) ([]*StoryLayer, error) {
	return s.store.ListStoryLayers(storyID)
}

func (s *StoryService) UpdateLayer(l *StoryLayer) error {
	return s.store.UpdateStoryLayer(l)
}

func (s *StoryService) DeleteLayer(id string) error {
	return s.store.DeleteStoryLayer(id)
}
