package task

import "time"

type StoryStore interface {
	CreateStory(st *Story) error
	GetStory(id string) (*Story, error)
	ListStories(status string) ([]*Story, error)
	UpdateStory(st *Story) error
	DeleteStory(id string) error
	SnoozeStory(id string, until time.Time) error
	UnsnoozeStory(id string) error
	CreateStoryTask(t *StoryTask) error
	GetStoryTask(id string) (*StoryTask, error)
	ListStoryTasks(storyID string) ([]*StoryTask, error)
	UpdateStoryTask(t *StoryTask) error
	DeleteStoryTask(id string) error
	AddTaskDep(taskID, dependsOnID string) error
	RemoveTaskDep(taskID, dependsOnID string) error
	CreateContextEntry(e *ContextEntry) error
	ListContextEntries(storyID string) ([]*ContextEntry, error)
	DeleteContextEntry(id string) error
	UpdateContextEntry(e *ContextEntry) error
	SetSetting(key, value string) error
	GetSetting(key string) (string, error)
	FindStoryByPRURL(prURL string) (string, error)
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

func (s *StoryService) Snooze(id string, until time.Time) error {
	return s.store.SnoozeStory(id, until)
}

func (s *StoryService) Unsnooze(id string) error {
	return s.store.UnsnoozeStory(id)
}

func (s *StoryService) AddTask(t *StoryTask) error {
	return s.store.CreateStoryTask(t)
}

func (s *StoryService) GetTask(id string) (*StoryTask, error) {
	return s.store.GetStoryTask(id)
}

func (s *StoryService) ListTasks(storyID string) ([]*StoryTask, error) {
	return s.store.ListStoryTasks(storyID)
}

func (s *StoryService) UpdateTask(t *StoryTask) error {
	return s.store.UpdateStoryTask(t)
}

func (s *StoryService) DeleteTask(id string) error {
	return s.store.DeleteStoryTask(id)
}

func (s *StoryService) AddTaskDep(taskID, dependsOnID string) error {
	return s.store.AddTaskDep(taskID, dependsOnID)
}

func (s *StoryService) RemoveTaskDep(taskID, dependsOnID string) error {
	return s.store.RemoveTaskDep(taskID, dependsOnID)
}

func (s *StoryService) AddContext(entry *ContextEntry) error {
	return s.store.CreateContextEntry(entry)
}

func (s *StoryService) ListContext(storyID string) ([]*ContextEntry, error) {
	return s.store.ListContextEntries(storyID)
}

func (s *StoryService) DeleteContext(id string) error {
	return s.store.DeleteContextEntry(id)
}

func (s *StoryService) UpdateContext(entry *ContextEntry) error {
	return s.store.UpdateContextEntry(entry)
}

func (s *StoryService) SetSetting(key, value string) error {
	return s.store.SetSetting(key, value)
}

func (s *StoryService) GetSetting(key string) (string, error) {
	return s.store.GetSetting(key)
}

func (s *StoryService) FindByPRURL(prURL string) (string, error) {
	return s.store.FindStoryByPRURL(prURL)
}
