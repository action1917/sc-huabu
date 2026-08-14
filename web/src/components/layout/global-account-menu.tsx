import { App, Popover } from "antd";
import { ChevronDown, Coins, LogIn, LogOut, Settings2, ShieldCheck, UserRound, UsersRound } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { Link, useNavigate } from "react-router";

import { IdentityProviderBadge } from "@/components/layout/identity-provider-badge";
import { useWalletBalance } from "@/hooks/use-wallet-balance";
import { applyUserSession } from "@/lib/user-session";
import { cn } from "@/lib/utils";
import { logout } from "@/services/api/auth";
import { useUserStore, type LocalUser } from "@/stores/use-user-store";

type GlobalAccountMenuProps = {
    compact?: boolean;
    className?: string;
};

const triggerClass =
    "flex h-9 min-w-9 items-center justify-center rounded-lg border border-border/65 bg-background/80 text-foreground/70 shadow-sm backdrop-blur-md transition hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

export function GlobalAccountMenu({ compact = false, className }: GlobalAccountMenuProps) {
    const user = useUserStore((state) => state.user);
    const hydrated = useUserStore((state) => state.hydrated);
    const features = useUserStore((state) => state.features);
    const navigate = useNavigate();
    const { message } = App.useApp();
    const [open, setOpen] = useState(false);
    const { availableMicrocredits } = useWalletBalance(user?.id, features.creditsEnabled);
    const balance = availableMicrocredits === null ? "--" : (availableMicrocredits / 1_000_000).toLocaleString("zh-CN", { maximumFractionDigits: 2 });

    const close = () => setOpen(false);
    const handleLogout = async () => {
        try {
            await logout();
            await applyUserSession({ user: null, systemChannels: [] });
            close();
            message.success("已退出登录");
            navigate("/login", { replace: true });
        } catch (error) {
            message.error(error instanceof Error ? error.message : "退出失败");
        }
    };

    if (!hydrated) return <span className={cn(triggerClass, "animate-pulse", compact ? "w-9" : "w-24", className)} aria-hidden />;
    if (!user) {
        return (
            <Link to="/login" className={cn(triggerClass, compact ? "w-9 px-0" : "gap-1.5 px-3", className)} aria-label="登录" title="登录">
                <LogIn className="size-4" />
                {!compact ? <span className="text-xs font-medium">登录</span> : null}
            </Link>
        );
    }

    return (
        <Popover
            trigger="click"
            placement="bottomRight"
            open={open}
            onOpenChange={setOpen}
            arrow={false}
            overlayClassName="app-global-account-popover"
            content={
                <div className="w-[min(320px,calc(100vw-24px))] py-1">
                    <div className="flex items-center gap-3 border-b border-border/65 px-2 pb-4 pt-1">
                        <UserAvatar user={user} className="size-11" />
                        <div className="min-w-0 flex-1">
                            <div className="flex min-w-0 items-center gap-1.5">
                                <strong className="truncate text-sm font-semibold">{user.displayName || user.username}</strong>
                                <IdentityProviderBadge user={user} />
                            </div>
                            <div className="mt-1 truncate text-[var(--fs-label)] text-foreground/42">账号 · {user.username}</div>
                        </div>
                        {features.creditsEnabled ? (
                            <Link to="/wallet" onClick={close} className="rounded-lg bg-foreground/[.055] px-2.5 py-1.5 text-right transition hover:bg-foreground/[.09]" title="进入积分中心">
                                <span className="block text-[var(--fs-micro)] text-foreground/42">可用积分</span>
                                <span className="mt-0.5 block text-xs font-semibold tabular-nums">{balance}</span>
                            </Link>
                        ) : null}
                    </div>

                    <nav className="space-y-0.5 py-2" aria-label="我的菜单">
                        {features.settingsEnabled ? <AccountMenuLink to="/settings" icon={<Settings2 />} label="设置" description="模型、存储与个人偏好" onNavigate={close} /> : null}
                        {features.teamsEnabled ? <AccountMenuLink to="/teams" icon={<UsersRound />} label="团队空间" description="创建、加入与共享团队资源" onNavigate={close} /> : null}
                        {features.creditsEnabled ? <AccountMenuLink to="/wallet" icon={<Coins />} label="积分中心" description="余额、邀请奖励与流水账单" onNavigate={close} /> : null}
                        {user.role === "admin" ? <AccountMenuLink to="/admin" icon={<ShieldCheck />} label="管理员后台" description="系统配置与运营管理" onNavigate={close} /> : null}
                    </nav>

                    <div className="border-t border-border/65 pt-2">
                        <button type="button" className="flex min-h-10 w-full items-center gap-3 rounded-lg px-3 text-left text-xs text-foreground/58 transition hover:bg-foreground/[.055] hover:text-foreground" onClick={() => void handleLogout()}>
                            <LogOut className="size-4" />
                            <span>退出登录</span>
                        </button>
                    </div>
                </div>
            }
        >
            <button type="button" className={cn(triggerClass, compact ? "w-9 px-0" : "w-9 px-0 xl:w-auto xl:max-w-[210px] xl:gap-2 xl:px-2.5", className)} aria-label="我的" aria-expanded={open} title="我的">
                <UserAvatar user={user} className="size-6" />
                {!compact ? (
                    <>
                        <span className="hidden max-w-24 truncate text-xs font-medium xl:inline">{user.displayName || user.username}</span>
                        <ChevronDown className={cn("hidden size-3.5 shrink-0 text-foreground/35 transition-transform xl:block", open && "rotate-180")} />
                    </>
                ) : null}
            </button>
        </Popover>
    );
}

function AccountMenuLink({ to, icon, label, description, onNavigate }: { to: string; icon: ReactNode; label: string; description: string; onNavigate: () => void }) {
    return (
        <Link to={to} onClick={onNavigate} className="group flex min-h-12 items-center gap-3 rounded-lg px-3 py-2 transition hover:bg-foreground/[.055]">
            <span className="grid size-8 shrink-0 place-items-center rounded-lg bg-foreground/[.045] text-foreground/55 transition group-hover:text-foreground [&_svg]:size-4">{icon}</span>
            <span className="min-w-0 flex-1">
                <span className="block text-xs font-medium text-foreground/80 group-hover:text-foreground">{label}</span>
                <span className="mt-0.5 block truncate text-[var(--fs-micro)] text-foreground/38">{description}</span>
            </span>
        </Link>
    );
}

function UserAvatar({ user, className }: { user: LocalUser; className: string }) {
    const [failed, setFailed] = useState(false);
    const avatarUrl = /^https?:\/\//i.test(user.avatarUrl || "") ? user.avatarUrl : "";

    useEffect(() => setFailed(false), [avatarUrl]);

    return (
        <span className={`relative grid shrink-0 place-items-center ${className}`}>
            <span className="grid size-full place-items-center overflow-hidden rounded-full bg-muted text-foreground/55">
                {avatarUrl && !failed ? <img src={avatarUrl} alt="" referrerPolicy="no-referrer" className="size-full object-cover" onError={() => setFailed(true)} /> : <UserRound className="size-[52%]" aria-hidden />}
            </span>
            <IdentityProviderBadge user={user} compact className="absolute -bottom-1 -right-1" />
        </span>
    );
}
