"use client";

import { Command } from "cmdk";
import * as Dialog from "@radix-ui/react-dialog";
import { useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { COMMAND_PALETTE_ITEMS } from "@/lib/commandPaletteItems";
import { useThemeToggle } from "./ThemeProvider";

type CommandPaletteContextValue = {
  open: boolean;
  setOpen: (open: boolean) => void;
};

const CommandPaletteContext = createContext<CommandPaletteContextValue | null>(null);

export function CommandPaletteProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  return (
    <CommandPaletteContext.Provider value={{ open, setOpen }}>
      {children}
      <CommandPaletteDialog />
    </CommandPaletteContext.Provider>
  );
}

function useCommandPalette() {
  const ctx = useContext(CommandPaletteContext);
  if (!ctx) throw new Error("useCommandPalette must be used within CommandPaletteProvider");
  return ctx;
}

function CommandPaletteDialog() {
  const { open, setOpen } = useCommandPalette();
  const router = useRouter();
  const { toggle: toggleTheme } = useThemeToggle();

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen(!open);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, setOpen]);

  const runItem = useCallback(
    (id: string, href?: string) => {
      setOpen(false);
      if (id === "toggle-theme") {
        toggleTheme();
        return;
      }
      if (href) router.push(href);
    },
    [router, setOpen, toggleTheme],
  );

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Portal>
        <Dialog.Overlay className="m3-cmdk-overlay" />
        <Dialog.Content className="m3-cmdk-content" aria-label="Command palette">
          <Dialog.Title className="sr-only">Command palette</Dialog.Title>
          <Command label="Command palette" className="m3-cmdk">
            <Command.Input placeholder="Search pages, actions…" className="m3-cmdk-input" />
            <Command.List className="m3-cmdk-list">
              <Command.Empty className="m3-cmdk-empty">No results found.</Command.Empty>
              {["Navigate", "Trading", "Actions"].map((group) => {
                const items = COMMAND_PALETTE_ITEMS.filter((i) => i.group === group);
                if (items.length === 0) return null;
                return (
                  <Command.Group key={group} heading={group} className="m3-cmdk-group">
                    {items.map((item) => (
                      <Command.Item
                        key={item.id}
                        value={[item.label, ...(item.keywords ?? [])].join(" ")}
                        className="m3-cmdk-item"
                        onSelect={() => runItem(item.id, item.href)}
                      >
                        <span>{item.label}</span>
                        {item.href ? <span className="m3-cmdk-item-hint">{item.href}</span> : null}
                      </Command.Item>
                    ))}
                  </Command.Group>
                );
              })}
            </Command.List>
            <div className="m3-cmdk-footer">
              <kbd>↑↓</kbd> navigate · <kbd>↵</kbd> select · <kbd>Esc</kbd> close
            </div>
          </Command>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

export function CommandPaletteTrigger() {
  const { setOpen } = useCommandPalette();
  return (
    <button type="button" className="m3-search-trigger" onClick={() => setOpen(true)} aria-label="Open command palette">
      <SearchIcon />
      <span>Search</span>
      <kbd className="m3-search-trigger__kbd">⌘K</kbd>
    </button>
  );
}

function SearchIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden>
      <circle cx="7" cy="7" r="4.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M10.5 10.5L14 14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}
