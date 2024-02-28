# mdrun.nvim

Markdown snippet runner for neovim.

## Features

- evaluate markdown codeblock in the language of your choice
- stream stdin and stdout to other codeblocks in real time
- define env-vars per markdown section
- run snippets either directly or in a docker/podman container
- define custom runners for languages not supported out of the box

## Installation



## Configuration Options in LUA

```lua
require('mdrun').setup({
  stop_signal = "SIGINT", -- Signal to send when attempting to stop a process. one of: [SIGKILL, SIGINT]

})
```

## Supported Languages

### Shell (bash, zsh, sh)

**Example 1:**

```sh CWD=/usr/share
echo "I'm in $(pwd)"
```

```out
I'm in /usr/share
```

**Example 2:**

**Config:**

| Key       | Default                 | Description                                                |
| --------- | ----------------------- | ---------------------------------------------------------- |
| CWD       | Neovims current workdir | working directory of the shell commands to run             |
| CONTAINER | None                    | Docker/podman container in which to run the shell command. |

### Golang (go)

**Example 1:**

```go ID=1703087980319
one := 1
two := 2
pi := math.Pi
fmt.Printf("One plus Two is: %d\n", one + two)
fmt.Printf("Pi is: %f\n", pi)
```

```out LAST_RUN=2023-12-20T17:00:24+01:00 SOURCE=1703087980319
One plus Two is: 3
Pi is: 3.141593
```

**Config:**

| Key       | Default | Description                                                                                       |
| --------- | ------- | ------------------------------------------------------------------------------------------------- |
| FULL_FILE | false   | Weather or not the codeblock specifies the full main.go or just the contents of the main function |

## Todo

| Language   | Done |
| ---------- | ---- |
| Bash/Shell | Yes  |
| C          | Yes  |
| C++        | Yes  |
| C#         | No   |
| Clojure    | No   |
| Elixir     | No   |
| F#         | No   |
| Go         | Yes  |
| Haskell    | Yes   |
| Java       | No   |
| JavaScript | No   |
| Julia      | No   |
| Lua        | Yes  |
| OCaml      | No   |
| Perl/Perl6 | No   |
| Python3    | No   |
| R          | No   |
| Ruby       | No   |
| Rust       | Yes   |
| Scala      | No   |
| TypeScript | No   |



```lua ID=1708957562401 IN_NVIM=true
f = vim.api.nvim_get_runtime_file("*/mdrun.lua", false)
vim.print(f)

```


```out SOURCE=1708957562401 EXIT_CODE=0 LAST_RUN=2024-02-26T15:33:12+01:00
Result: <nil>
```
