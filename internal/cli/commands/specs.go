package commands

type DomainSpec struct {
	Name    string
	Actions []string
}

func DomainSpecs() []DomainSpec {
	return []DomainSpec{
		{Name: "bridge", Actions: []string{"discover", "add", "link", "list", "show", "rename", "remove", "health", "capabilities"}},
		{Name: "device", Actions: []string{"search", "list", "show", "identify", "rename", "delete"}},
		{Name: "light", Actions: []string{"list", "show", "on", "off", "toggle", "set", "effect", "flash"}},
		{Name: "room", Actions: []string{"list", "create", "update", "delete", "assign", "unassign"}},
		{Name: "zone", Actions: []string{"list", "create", "update", "delete", "assign", "unassign"}},
		{Name: "scene", Actions: []string{"list", "show", "create", "update", "delete", "activate", "clone"}},
		{Name: "automation", Actions: []string{"list", "show", "create", "update", "delete", "enable", "disable", "run"}},
		{Name: "sensor", Actions: []string{"list", "show", "rename", "sensitivity", "enable", "disable"}},
		{Name: "update", Actions: []string{"list", "check", "install", "status"}},
		{Name: "backup", Actions: []string{"export", "import", "diff"}},
		{Name: "diagnose", Actions: []string{"ping", "latency", "events", "logs"}},
		{Name: "api", Actions: []string{"get", "post", "put", "delete"}},
	}
}

func EntertainmentAreaActions() []string {
	return []string{"list", "create", "update", "delete"}
}

func EntertainmentSessionActions() []string {
	return []string{"start", "stop"}
}
