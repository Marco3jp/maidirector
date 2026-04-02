package task

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
	CreateContextEntry(e *ContextEntry) error
	ListContextEntries(storyID string) ([]*ContextEntry, error)
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

func (s *StoryService) AddContext(entry *ContextEntry) error {
	return s.store.CreateContextEntry(entry)
}

func (s *StoryService) ListContext(storyID string) ([]*ContextEntry, error) {
	return s.store.ListContextEntries(storyID)
}
