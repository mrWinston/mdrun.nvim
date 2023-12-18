

local M = {}

M.config = {}

M.setup = function(opts)
	M.config = vim.tbl_deep_extend("force", M.config, opts or {})
	-- make sure query output is highlighted with markdown
	--vim.treesitter.language.register("bash", "zsh")

	if vim.g.loaded_mdrun_nvim then
		return
	end
	vim.cmd([[
    function! s:RequireMdrun(host) abort
      return jobstart(['mdrun.nvim'], {'rpc': v:true})
    endfunction
  
    call remote#host#Register('mdrun', 'x', function('s:RequireMdrun'))

    call remote#host#RegisterPlugin('mdrun', '0', [
    \ {'type': 'function', 'name': 'MdrunRunCodeblock', 'sync': 0, 'opts': {}},
    \ ])
  ]])
	vim.g.loaded_mdrun_nvim = true
end

return M
