package logs

type Hook interface {
	Levels() []Level
	Fire(event *LogEvent) error
}

type LevelHooks map[Level][]Hook

func (hooks LevelHooks) Add(hook Hook) {
	for _, level := range hook.Levels() {
		hooks[level] = append(hooks[level], hook)
	}
}

func (hooks LevelHooks) Fire(level Level, event *LogEvent) error {
	for _, hook := range hooks[level] {
		if err := hook.Fire(event); err != nil {
			return err
		}
	}
	return nil
}
