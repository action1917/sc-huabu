import { App, Popover } from "antd";
import { ChevronRight, LogIn, LogOut, ShieldCheck, UserRound } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { Link, useNavigate } from "react-router";

import { IdentityProviderBadge } from "@/components/layout/identity-provider-badge";
import { useWalletBalance } from "@/hooks/use-wallet-balance";
import { applyUserSession } from "@/lib/user-session";
import { cn } from "@/lib/utils";
import { logout } from "@/services/api/auth";
import { useUserStore, type LocalUser } from "@/stores/use-user-store";

type WorkspaceSidebarFooterProps = {
    expandedClassName: string;
    collapsedClassName: string;
    accountClassName: string;
};

export function WorkspaceSidebarFooter({ expandedClassName, collapsedClassName, accountClassName }: WorkspaceSidebarFooterProps) {
    const user = useUserStore((state) => state.user);
    const hydrated = useUserStore((state) => state.hydrated);
    const creditsEnabled = useUserStore((state) => state.features.creditsEnabled);
    const navigate = useNavigate();
    const { message } = App.useApp();
    const [menuOpen, setMenuOpen] = useState(false);
    const { availableMicrocredits } = useWalletBalance(user?.id, creditsEnabled);
    const balance = availableMicrocredits === null
        ? "--"
        : (availableMicrocredits / 1_000_000).toLocaleString("zh-CN", { maximumFractionDigits: 2 });

    const handleLogout = async () => {
        try {
            await logout();
            await applyUserSession({ user: null, systemChannels: [] });
            setMenuOpen(false);
            message.success("已退出登录");
            navigate("/login", { replace: true });
        } catch (error) {
            message.error(error instanceof Error ? error.message : "退出失败");
        }
    };

    if (!hydrated) return <div className="h-24 animate-pulse rounded-md bg-foreground/[.035]" />;

    return (
        <div>
            {user ? (
                <Popover
                    trigger="click"
                    placement="rightBottom"
                    open={menuOpen}
                    onOpenChange={setMenuOpen}
                    content={(
                        <div className="w-[232px] py-0.5">
                            <div className="flex items-center gap-3 border-b border-border/65 px-1 pb-3">
                                <UserAvatar user={user} className="size-8" />
                                <div className="min-w-0 flex-1">
                                    <div className="flex min-w-0 items-center gap-1.5"><span className="truncate text-sm font-medium">{user.displayName || user.username}</span><IdentityProviderBadge user={user} /></div>
                                    {creditsEnabled ? <div className="mt-0.5 truncate text-[var(--fs-label)] tabular-nums text-foreground/45">可用 {balance} 积分</div> : null}
                                </div>
                            </div>

                            {user.role === "admin" ? (
                                <nav className="py-2" aria-label="管理工具">
                                    <MenuLink to="/admin" icon={<ShieldCheck />} label="管理员后台" onNavigate={() => setMenuOpen(false)} />
                                </nav>
                            ) : null}

                            <button type="button" className="flex h-9 w-full items-center gap-2 rounded px-2 text-xs text-foreground/55 hover:bg-foreground/[.055] hover:text-foreground" onClick={() => void handleLogout()}><LogOut className="size-3.5" />退出登录</button>
                        </div>
                    )}
                >
                    <button type="button" className={cn("flex min-h-10 w-full min-w-0 items-center overflow-hidden rounded-md text-left transition-colors hover:bg-foreground/[.045]", accountClassName)} title={creditsEnabled ? `${user.displayName || user.username} · ${balance} 积分` : user.displayName || user.username}>
                        <UserAvatar user={user} className="size-7" />
                        <span className={cn("min-w-0 flex-1 flex-col", expandedClassName)}>
                            <span className="truncate text-xs font-medium">{user.displayName || user.username}</span>
                            {creditsEnabled ? <span className="mt-0.5 block truncate text-[var(--fs-micro)] tabular-nums text-foreground/42">{balance} 积分</span> : null}
                        </span>
                        <ChevronRight className={cn("size-3.5 shrink-0 text-foreground/30", expandedClassName)} />
                    </button>
                </Popover>
            ) : (
                <Link to="/login" className={cn("flex h-10 items-center rounded-md text-xs text-foreground/65 hover:bg-foreground/[.05] hover:text-foreground", collapsedClassName)} title="登录">
                    <LogIn className="size-4 shrink-0" /><span className={expandedClassName}>登录</span>
                </Link>
            )}
        </div>
    );
}

function MenuLink({ to, icon, label, onNavigate }: { to: string; icon: ReactNode; label: string; onNavigate: () => void }) {
    return <Link to={to} onClick={onNavigate} className="flex h-9 items-center gap-2.5 rounded px-2 text-xs text-foreground/62 hover:bg-foreground/[.055] hover:text-foreground [&_svg]:size-3.5 [&_svg]:shrink-0">{icon}<span className="flex-1">{label}</span><ChevronRight className="!size-3 text-foreground/25" /></Link>;
}

function UserAvatar({ user, className }: { user: LocalUser; className: string }) {
    const [failed, setFailed] = useState(false);
    const avatarUrl = /^https?:\/\//i.test(user.avatarUrl || "") ? user.avatarUrl : "";

    useEffect(() => setFailed(false), [avatarUrl]);

    return (
        <span className={`relative grid shrink-0 place-items-center ${className}`}>
            <span className="grid size-full place-items-center overflow-hidden rounded-full bg-muted text-foreground/55">
                {avatarUrl && !failed ? (
                    <img src={avatarUrl} alt="" referrerPolicy="no-referrer" className="size-full object-cover" onError={() => setFailed(true)} />
                ) : (
                    <UserRound className="size-[52%]" aria-hidden />
                )}
            </span>
            <IdentityProviderBadge user={user} compact className="absolute -bottom-1 -right-1" />
        </span>
    );
}
