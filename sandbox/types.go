package sandbox

type Sysinfo struct {
	Base      string
	Flavor    string
	Version   string
	Adjacents []string
}

type Sandbox struct {
	ID      string
	Name    string
	Command []string
}
