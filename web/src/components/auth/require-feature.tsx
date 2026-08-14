import { useEffect, useState, type ReactNode } from "react";
import { Button } from "antd";
import { useLocation, useNavigate } from "react-router";

import { WorkspacePage } from "@/components/layout/workspace-page";
import { WorkspaceErrorState, WorkspaceLoadingState, WorkspaceState } from "@/components/layout/workspace-state";
import { refreshFeatureAvailability } from "@/lib/user-session";
import { useUserStore, type FeatureKey } from "@/stores/use-user-store";

const featureNames: Record<FeatureKey, string> = {
    creationEnabled: "创作",
    shortDramaEnabled: "短剧创作",
    inspirationsEnabled: "灵感广场",
    canvasEnabled: "画布",
    taskCenterEnabled: "任务中心",
    assetsEnabled: "资产",
    skillsEnabled: "技能库",
    teamsEnabled: "团队空间",
    creditsEnabled: "积分中心",
    settingsEnabled: "设置",
};

export function RequireFeature({ feature, children }: { feature: FeatureKey; children: ReactNode }) {
    const location = useLocation();
    const navigate = useNavigate();
    const features = useUserStore((state) => state.features);
    const [checking, setChecking] = useState(true);
    const [error, setError] = useState("");

    useEffect(() => {
        let cancelled = false;
        setChecking(true);
        setError("");
        refreshFeatureAvailability()
            .catch((reason) => {
                if (!cancelled) setError(reason instanceof Error ? reason.message : "读取功能开放状态失败");
            })
            .finally(() => {
                if (!cancelled) setChecking(false);
            });
        return () => {
            cancelled = true;
        };
    }, [location.pathname]);

    if (checking) return <WorkspacePage><WorkspaceLoadingState label="正在确认功能状态" detail={featureNames[feature]} rows={3} /></WorkspacePage>;
    if (error) return <WorkspacePage><WorkspaceErrorState title="无法确认功能状态" description={error} actionLabel="返回创作台" onRetry={() => navigate("/create", { replace: true })} /></WorkspacePage>;
    if (!features[feature]) {
        return (
            <WorkspacePage>
                <WorkspaceState icon="empty" title={`${featureNames[feature]}暂未开放`} description="当前功能已由平台管理员关闭。" action={<Button type="primary" onClick={() => navigate("/create", { replace: true })}>返回创作台</Button>} />
            </WorkspacePage>
        );
    }
    return children;
}
