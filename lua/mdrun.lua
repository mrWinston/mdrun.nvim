local M = {}

-- default config
M.config = {
	stop_signal = "SIGINT", -- or SIGKILL
  docker_runtime = "podman", -- or docker
	runner_configs = {
		c = {
			type = "CompiledRunner",
			languages = { "c" },
      image = "gcc",
			config = {
				compiler = "gcc",
				output_flag = "-o",
				file_name = "main.c",
			},
		},
		cpp = {
			type = "CompiledRunner",
			languages = { "cpp", "c++" },
      image = "gcc",
			config = {
				compiler = "g++",
				output_flag = "-o",
				file_name = "main.cpp",
			},
		},
		golang = {
			type = "GoRunner",
			languages = { "go" },
      image = "golang",
			config = {
				use_gomacro = true,
			},
		},
		haskell = {
			type = "CompiledRunner",
			languages = { "haskell" },
      image = "haskell",
			config = {
				compiler = "ghc",
				output_flag = "-o",
				file_name = "main.hs",
			},
		},
		java = {
			type = "JavaRunner",
			languages = { "java" },
      image = "eclipse-temurin",
			config = {
				use_jshell = true,
			},
		},
		javascript = {
			type = "InterpretedRunner",
			languages = { "javascript", "js" },
      image = "denoland/deno:alpine",
			config = {
				interpreter = "deno run",
				file_name = "main.js",
			},
		},
		lua = {
			type = "LuaRunner",
			languages = { "lua" },
      image = "nickblah/lua",
      config = {
				in_nvim = true,
			},
		},
		python = {
			type = "InterpretedRunner",
			languages = { "python", "py" },
      image = "python",
			config = {
				interpreter = "python",
				file_name = "main.py",
			},
		},
		rust = {
			type = "CompiledRunner",
			languages = { "rust" },
      image = "rust",
			config = {
				compiler = "rustc",
				output_flag = "-o",
				file_name = "main.rs",
			},
		},
		shell = {
			type = "ShellRunner",
			languages = { "sh", "zsh", "bash" },
			config = {
				default_shell = "zsh",
			},
		},
		typescript = {
			type = "InterpretedRunner",
			languages = { "typescript" },
      image = "denoland/deno:alpine",
			config = {
				interpreter = "deno run",
				file_name = "main.ts",
			},
		},
	},
}

M.setup = function(opts)
	M.config = vim.tbl_deep_extend("force", M.config, opts or {})
	-- make sure query output is highlighted with markdown
	-- vim.treesitter.language.register("bash", "zsh")
  
  if vim.g.loaded_mdrun_nvim then
		return
	end

	vim.cmd([[
    function! s:RequireMdrun(host) abort
      return jobstart(['mdrun.nvim'], {'rpc': v:true})
    endfunction
  
    call remote#host#Register('mdrun', 'x', function('s:RequireMdrun'))

    call remote#host#RegisterPlugin('mdrun', '0', [
    \ {'type': 'autocmd', 'name': 'BufReadPost', 'sync': 0, 'opts': {'group': 'mdrun', 'pattern': '*.md'}},
    \ {'type': 'function', 'name': 'MdrunConfigure', 'sync': 1, 'opts': {}},
    \ {'type': 'function', 'name': 'MdrunKillCodeblock', 'sync': 0, 'opts': {}},
    \ {'type': 'function', 'name': 'MdrunRunCodeblock', 'sync': 0, 'opts': {}},
    \ ])
  ]])
	vim.g.loaded_mdrun_nvim = true

  vim.fn.MdrunConfigure(vim.json.encode(M.config))
end

return M
