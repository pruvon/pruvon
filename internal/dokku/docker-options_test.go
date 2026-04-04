package dokku

import "testing"

func TestParseOptionsList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"   ", []string{}},
		{"--ulimit nofile=12", []string{"--ulimit nofile=12"}},
		{"--shm-size=256m --restart always", []string{"--shm-size=256m", "--restart always"}},
		{"--env FOO=bar --env BAZ=qux", []string{"--env FOO=bar", "--env BAZ=qux"}},
	}
	for _, c := range cases {
		got := parseOptionsList(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("len mismatch for %q: got %d want %d", c.in, len(got), len(c.want))
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("[%q] got[%d]=%q want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestParseDockerOptions(t *testing.T) {
	input := "Docker options build: --ulimit nofile=12 --shm-size=256m\n" +
		"Docker options deploy: --restart always\n" +
		"Docker options run: --env FOO=bar --env BAZ=qux\n"
	got := ParseDockerOptions(input)
	if len(got.Build) != 2 || got.Build[0] != "--ulimit nofile=12" || got.Build[1] != "--shm-size=256m" {
		t.Fatalf("unexpected build: %#v", got.Build)
	}
	if len(got.Deploy) != 1 || got.Deploy[0] != "--restart always" {
		t.Fatalf("unexpected deploy: %#v", got.Deploy)
	}
	if len(got.Run) != 2 || got.Run[0] != "--env FOO=bar" || got.Run[1] != "--env BAZ=qux" {
		t.Fatalf("unexpected run: %#v", got.Run)
	}
}
