BINARY      := ticky
INSTALL_DIR := $(HOME)/.local/bin
BUILD_DIR   := bin
RELEASES_DIR := releases
COMMIT      := $(shell git rev-parse HEAD 2>/dev/null || printf dev)

BASHRC := $(HOME)/.bashrc
ZSHRC  := $(HOME)/.zshrc
FISHRC := $(HOME)/.config/fish/config.fish
PWSHRC := $(HOME)/.config/powershell/Microsoft.PowerShell_profile.ps1

# tmux config — prefer XDG location, fall back to legacy ~/.tmux.conf
TMUXCFG := $(shell [ -f $(HOME)/.config/tmux/tmux.conf ] && echo $(HOME)/.config/tmux/tmux.conf || echo $(HOME)/.tmux.conf)

.PHONY: all build build-all install install-shell uninstall test test-all clean

all: build

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$(COMMIT)" -o $(BUILD_DIR)/$(BINARY) .
	@echo "Built: $(BUILD_DIR)/$(BINARY)"

build-all:
	@mkdir -p $(RELEASES_DIR)/linux/amd64 $(RELEASES_DIR)/linux/arm64 $(RELEASES_DIR)/darwin/amd64 $(RELEASES_DIR)/darwin/arm64 $(RELEASES_DIR)/windows
	@echo "Building linux/amd64..."
	GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$(COMMIT)" -o $(RELEASES_DIR)/linux/amd64/$(BINARY) .
	@echo "Building linux/arm64..."
	GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$(COMMIT)" -o $(RELEASES_DIR)/linux/arm64/$(BINARY) .
	@echo "Building darwin/amd64..."
	GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$(COMMIT)" -o $(RELEASES_DIR)/darwin/amd64/$(BINARY) .
	@echo "Building darwin/arm64..."
	GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$(COMMIT)" -o $(RELEASES_DIR)/darwin/arm64/$(BINARY) .
	@echo "Building windows/amd64..."
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$(COMMIT)" -o $(RELEASES_DIR)/windows/$(BINARY).exe .
	@echo "Pre-built binaries written to $(RELEASES_DIR)/"

install:
	@mkdir -p $(INSTALL_DIR)
	@if command -v go >/dev/null 2>&1; then \
		echo "==> Go found - building ticky from source..."; \
		mkdir -p $(BUILD_DIR); \
		go build -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$(COMMIT)" -o $(BUILD_DIR)/$(BINARY) . || exit 1; \
		cp $(BUILD_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY); \
		echo "    Built and installed from source."; \
	else \
		echo "==> Go not found - installing pre-built binary from releases/..."; \
		OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
		ARCH=$$(uname -m); \
		case "$$ARCH" in x86_64|amd64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; *) echo "ERROR: Unsupported architecture: $$ARCH"; exit 1 ;; esac; \
		if [ "$$OS" = "darwin" ]; then RELEASE_BIN="$(RELEASES_DIR)/darwin/$$ARCH/$(BINARY)"; elif [ "$$OS" = "linux" ]; then RELEASE_BIN="$(RELEASES_DIR)/linux/$$ARCH/$(BINARY)"; else echo "ERROR: Unsupported OS: $$OS"; exit 1; fi; \
		if [ ! -f "$$RELEASE_BIN" ]; then echo "ERROR: Pre-built binary not found at $$RELEASE_BIN"; echo "       Please install Go (https://go.dev/dl/) and re-run, or ask a developer to run 'make build-all' and commit the releases/ folder."; exit 1; fi; \
		cp "$$RELEASE_BIN" $(INSTALL_DIR)/$(BINARY); \
		chmod +x $(INSTALL_DIR)/$(BINARY); \
		echo "    Installed pre-built binary."; \
	fi
	@echo "Installed: $(INSTALL_DIR)/$(BINARY)"
	@echo ""
	@$(MAKE) --no-print-directory install-shell

# ── Shell prompt integration ────────────────────────────────────────────────
#
# tmux is handled first and independently — it provides live per-second
# updates in the status bar regardless of what is running in the active pane.
#
# Shell prompt fallback chain (for when tmux is not available):
#   bash   → PS1 prefix
#   zsh    → RPROMPT
#   fish   → fish_right_prompt
#   pwsh   → prompt function
#
# Note: if starship is in use, the PS1/RPROMPT methods below will be
# overridden on every prompt draw. Use the tmux integration for live updates
# instead — it works regardless of which prompt system is active.
#
install-shell:
	@echo "  Setting up shell prompt integration..."; \
	echo ""; \
	\
	tmux_found=0; \
	if [ -f "$(TMUXCFG)" ]; then \
		tmux_found=1; \
		if grep -q "ticky shell integration" "$(TMUXCFG)" 2>/dev/null; then \
			echo "  tmux: ticky already integrated — skipping"; \
		else \
			printf '\n# ticky shell integration\nset -g status-interval 1\nset -g status-right-length 120\nset -g status-right "#(ticky --status 2>/dev/null)  #[fg=blue]#{?client_prefix,PREFIX ,}#[fg=brightblack]#h "\n' >> "$(TMUXCFG)"; \
			echo "  tmux: added live status bar to $(TMUXCFG)"; \
			if [ -n "$$TMUX" ]; then \
				tmux source-file "$(TMUXCFG)" 2>/dev/null && echo "  tmux: config reloaded — status bar is live"; \
			else \
				echo "  tmux: run 'tmux source-file $(TMUXCFG)' to apply without restarting tmux"; \
			fi; \
		fi; \
		echo ""; \
	fi; \
	\
	if [ "$$tmux_found" -eq 0 ] && [ -f "$(BASHRC)" ]; then \
		if grep -q "ticky shell integration" "$(BASHRC)" 2>/dev/null; then \
			echo "  bash: ticky already integrated — skipping"; \
		else \
			printf '\n# ticky shell integration\nexport PS1='"'"'$$(ticky --status && echo " ")'"'"'"$$PS1\n' >> "$(BASHRC)"; \
			echo "  bash: added ticky --status to PS1 in $(BASHRC)"; \
		fi; \
	elif [ "$$tmux_found" -eq 0 ] && [ -f "$(ZSHRC)" ]; then \
		if grep -q "ticky shell integration" "$(ZSHRC)" 2>/dev/null; then \
			echo "  zsh: ticky already integrated — skipping"; \
		else \
			printf '\n# ticky shell integration\nRPROMPT='"'"'$$(ticky --status)'"'"'\n' >> "$(ZSHRC)"; \
			echo "  zsh: added ticky --status to RPROMPT in $(ZSHRC)"; \
		fi; \
	elif [ "$$tmux_found" -eq 0 ] && [ -f "$(FISHRC)" ]; then \
		if grep -q "ticky shell integration" "$(FISHRC)" 2>/dev/null; then \
			echo "  fish: ticky already integrated — skipping"; \
		else \
			printf '\n# ticky shell integration\nfunction fish_right_prompt\n    ticky --status\nend\n' >> "$(FISHRC)"; \
			echo "  fish: added ticky --status to fish_right_prompt in $(FISHRC)"; \
		fi; \
	elif [ "$$tmux_found" -eq 0 ] && [ -f "$(PWSHRC)" ]; then \
		if grep -q "ticky shell integration" "$(PWSHRC)" 2>/dev/null; then \
			echo "  pwsh: ticky already integrated — skipping"; \
		else \
			printf '\n# ticky shell integration\nfunction prompt {\n    $$s = ticky --status 2>$$null\n    if ($$s) { Write-Host "$$s " -NoNewline -ForegroundColor Blue }\n    "PS $$(Get-Location)> "\n}\n' >> "$(PWSHRC)"; \
			echo "  pwsh: added ticky prompt function to $(PWSHRC)"; \
		fi; \
	elif [ "$$tmux_found" -eq 0 ]; then \
		echo "  No supported shell config found."; \
		echo "  See the README for manual prompt integration instructions."; \
	fi; \
	echo ""; \
	echo "  To show the active task in your prompt, enable these in ticky.toml"; \
	echo "  (press 'o' inside ticky to open it):"; \
	echo "    show_task_name = true"; \
	echo "    show_time_left = true"; \
	echo ""; \
	echo "  Reload your shell, then run: ticky"

# ── Uninstall ───────────────────────────────────────────────────────────────
uninstall:
	@rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Removed $(INSTALL_DIR)/$(BINARY)"
	@echo ""
	@# Remove tmux integration
	@if [ -f "$(TMUXCFG)" ] && grep -q "ticky shell integration" "$(TMUXCFG)" 2>/dev/null; then \
		python3 -c "\
import re; \
content = open('$(TMUXCFG)').read(); \
cleaned = re.sub(r'\n# ticky shell integration\nset -g status-interval[^\n]*\nset -g status-right-length[^\n]*\nset -g status-right[^\n]*\n', '\n', content); \
cleaned = re.sub(r'\n# ticky shell integration\n', '\n', cleaned); \
open('$(TMUXCFG)', 'w').write(cleaned.rstrip() + '\n')" 2>/dev/null \
		&& echo "  Removed ticky integration from $(TMUXCFG)"; \
		if [ -n "$$TMUX" ]; then \
			tmux source-file "$(TMUXCFG)" 2>/dev/null && echo "  tmux: config reloaded"; \
		fi; \
	fi
	@# Remove bash integration
	@if [ -f "$(BASHRC)" ] && grep -q "ticky shell integration" "$(BASHRC)" 2>/dev/null; then \
		python3 -c "\
import re; \
content = open('$(BASHRC)').read(); \
cleaned = re.sub(r'\n# ticky shell integration\nexport PS1[^\n]*\n', '\n', content); \
cleaned = re.sub(r'\n# ticky shell integration\nfunction _ticky_precmd[^}]*\}\nstarship_precmd_user_func[^\n]*\n', '\n', cleaned); \
cleaned = re.sub(r'\n# ticky shell integration\n', '\n', cleaned); \
open('$(BASHRC)', 'w').write(cleaned)" 2>/dev/null \
		&& echo "  Removed ticky integration from $(BASHRC)"; \
	fi
	@# Remove zsh integration
	@if [ -f "$(ZSHRC)" ] && grep -q "ticky shell integration" "$(ZSHRC)" 2>/dev/null; then \
		python3 -c "\
import re; \
content = open('$(ZSHRC)').read(); \
cleaned = re.sub(r'\n# ticky shell integration\nRPROMPT[^\n]*\n', '\n', content); \
open('$(ZSHRC)', 'w').write(cleaned)" 2>/dev/null \
		&& echo "  Removed ticky integration from $(ZSHRC)"; \
	fi
	@# Remove fish integration
	@if [ -f "$(FISHRC)" ] && grep -q "ticky shell integration" "$(FISHRC)" 2>/dev/null; then \
		python3 -c "\
import re; \
content = open('$(FISHRC)').read(); \
cleaned = re.sub(r'\n# ticky shell integration\nfunction fish_right_prompt\n    ticky --status\nend\n', '\n', content); \
open('$(FISHRC)', 'w').write(cleaned)" 2>/dev/null \
		&& echo "  Removed ticky integration from $(FISHRC)"; \
	fi
	@# Remove pwsh integration
	@if [ -f "$(PWSHRC)" ] && grep -q "ticky shell integration" "$(PWSHRC)" 2>/dev/null; then \
		python3 -c "\
import re; \
content = open('$(PWSHRC)').read(); \
cleaned = re.sub(r'\n# ticky shell integration\nfunction prompt \{[^}]*\}\n', '\n', content); \
open('$(PWSHRC)', 'w').write(cleaned)" 2>/dev/null \
		&& echo "  Removed ticky integration from $(PWSHRC)"; \
	fi
	@echo ""
	@echo "Your config and task data have been left in place:"
	@echo "  Linux:  $$HOME/.config/delbysoft/"
	@echo "  macOS:  $$HOME/Library/Application Support/delbysoft/"
	@echo "Delete that directory manually if you want a full clean uninstall."

test:
	go test ./internal/... -timeout 30s

test-all: test
	go test ./... -timeout 30s

clean:
	rm -rf $(BUILD_DIR)
