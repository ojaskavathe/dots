-- Treesitter incremental selection (replaces nvim-treesitter-textsubjects)
-- v. to smart select, keep pressing . to expand, , to shrink
-- v; for container outer, vi; for container inner

local node_stack = {}

local function select_node(node)
	if not node then
		return
	end
	local sr, sc, er, ec = node:range()
	vim.fn.setpos("'<", { 0, sr + 1, sc + 1, 0 })
	vim.fn.setpos("'>", { 0, er + 1, ec, 0 })
	vim.cmd("normal! gv")
end

local function smart_select()
	local current = node_stack[#node_stack]
	if not current then
		local node = vim.treesitter.get_node()
		if not node then
			return
		end
		node_stack = { node }
		select_node(node)
	else
		local parent = current:parent()
		if parent then
			table.insert(node_stack, parent)
			select_node(parent)
		end
	end
end

local function shrink()
	if #node_stack > 1 then
		table.remove(node_stack)
		select_node(node_stack[#node_stack])
	end
end

-- Find the nearest named ancestor that is a "container" (function, class, etc.)
local container_types = {
	["function_definition"] = true,
	["function_declaration"] = true,
	["method_definition"] = true,
	["method_declaration"] = true,
	["class_definition"] = true,
	["class_declaration"] = true,
	["module"] = true,
	["block"] = true,
	["if_statement"] = true,
	["for_statement"] = true,
	["while_statement"] = true,
	["do_block"] = true,
	["function_item"] = true, -- rust
	["impl_item"] = true, -- rust
}

local function find_container(node)
	while node do
		if container_types[node:type()] then
			return node
		end
		node = node:parent()
	end
end

local function container_outer()
	local node = vim.treesitter.get_node()
	local container = find_container(node)
	if container then
		node_stack = { container }
		select_node(container)
	end
end

local function container_inner()
	local node = vim.treesitter.get_node()
	local container = find_container(node)
	if not container then
		return
	end
	-- Select the body: the last named child is usually the body
	local body = nil
	for child in container:iter_children() do
		if child:named() then
			body = child
		end
	end
	if body then
		node_stack = { body }
		select_node(body)
	end
end

-- Reset stack when leaving visual mode
vim.api.nvim_create_autocmd("ModeChanged", {
	pattern = "[vV\x16]*:n",
	callback = function()
		node_stack = {}
	end,
})

return {
	{
		"nvim-treesitter",
		for_cat = "general.always",
		event = "DeferredUIEnter",
		after = function(_)
			vim.keymap.set("x", ".", smart_select, { desc = "Expand treesitter selection" })
			vim.keymap.set("x", ",", shrink, { desc = "Shrink treesitter selection" })
			vim.keymap.set("x", ";", container_outer, { desc = "Select container (outer)" })
			vim.keymap.set("x", "i;", container_inner, { desc = "Select container (inner)" })
		end,
	},
}
