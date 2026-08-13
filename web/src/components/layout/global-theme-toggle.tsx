import { Tooltip } from "antd";
import { Moon, Sun } from "lucide-react";

import { cn } from "@/lib/utils";
import { useThemeStore } from "@/stores/use-theme-store";

export function GlobalThemeToggle({ className }: { className?: string }) {
    const theme = useThemeStore((state) => state.theme);
    const setTheme = useThemeStore((state) => state.setTheme);
    const targetTheme = theme === "dark" ? "light" : "dark";
    const label = targetTheme === "dark" ? "切换到深色模式" : "切换到浅色模式";

    return (
        <Tooltip title={label} placement="bottom">
            <button
                type="button"
                onClick={() => setTheme(targetTheme)}
                className={cn("app-global-theme-toggle grid size-9 place-items-center rounded-lg border border-border/65 bg-background/80 text-foreground/65 shadow-sm backdrop-blur-md transition-colors hover:bg-foreground/[.07] hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring", className)}
                aria-label={label}
            >
                {theme === "dark" ? <Sun className="size-4" /> : <Moon className="size-4" />}
            </button>
        </Tooltip>
    );
}
