import { Tooltip } from "antd";
import { History } from "lucide-react";
import { useLocation, useNavigate } from "react-router";

import { GlobalThemeToggle } from "@/components/layout/global-theme-toggle";
import { SystemAnnouncementCenter } from "@/components/layout/system-announcement-center";
import { cn } from "@/lib/utils";
import { useUserStore } from "@/stores/use-user-store";

type GlobalWorkspaceActionsProps = {
    className?: string;
    buttonClassName?: string;
};

const defaultButtonClass = "grid size-9 place-items-center rounded-lg border border-border/65 bg-background/80 text-foreground/65 shadow-sm backdrop-blur-md transition hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

export function GlobalWorkspaceActions({ className, buttonClassName }: GlobalWorkspaceActionsProps) {
    const { pathname } = useLocation();
    const navigate = useNavigate();
    const user = useUserStore((state) => state.user);
    const actionClass = cn(defaultButtonClass, buttonClassName);

    const openHistory = () => {
        if (pathname === "/create") {
            window.dispatchEvent(new CustomEvent("creation:open-history"));
            return;
        }
        navigate("/create?history=1");
    };

    return (
        <div className={cn("flex items-center gap-1.5", className)}>
            <Tooltip title="历史对话">
                <button type="button" className={actionClass} onClick={openHistory} aria-label="历史对话"><History className="size-4" /></button>
            </Tooltip>
            {user ? <SystemAnnouncementCenter userId={user.id} className={cn("relative", actionClass)} staticMotion /> : null}
            <GlobalThemeToggle className={actionClass} />
        </div>
    );
}
