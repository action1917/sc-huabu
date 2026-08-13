import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Input, Select, Spin } from "antd";
import { Coins, Copy, Film, Heart, Image as ImageIcon, RotateCcw, Search, Sparkles, WandSparkles } from "lucide-react";
import { useNavigate } from "react-router";

import { formatCredits } from "@/constant/credits";
import { saveInspirationReplica } from "@/lib/inspiration-replica";
import { listInspirations, replicateInspiration, setInspirationFavorite, unlockInspiration, type InspirationItem } from "@/services/api/inspirations";
import { CollectionGrid, PageHeader, WorkspacePage } from "@/components/layout/workspace-page";
import { WorkspaceErrorState, WorkspaceLoadingState, WorkspaceState } from "@/components/layout/workspace-state";

export default function InspirationsPage() {
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const { message, modal } = App.useApp();
    const [kind, setKind] = useState("all");
    const [keyword, setKeyword] = useState("");
    const [scope, setScope] = useState<"public" | "favorites">("public");
    const query = useQuery({ queryKey: ["inspirations", scope, kind, keyword], queryFn: () => listInspirations({ scope, kind, keyword: keyword.trim(), limit: 60 }) });
    const mutation = useMutation({
        mutationFn: replicateInspiration,
        onSuccess: ({ replica }) => {
            saveInspirationReplica(replica);
            void queryClient.invalidateQueries({ queryKey: ["inspirations"] });
            message.success("灵感已复制到创作页");
            navigate("/create?from=inspiration");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "复制灵感失败"),
    });
    const unlockMutation = useMutation({ mutationFn: unlockInspiration });
    const favoriteMutation = useMutation({
        mutationFn: ({ id, favorite }: { id: string; favorite: boolean }) => setInspirationFavorite(id, favorite),
        onSuccess: (_, variables) => {
            void queryClient.invalidateQueries({ queryKey: ["inspirations"] });
            message.success(variables.favorite ? "已收藏到我的收藏" : "已取消收藏");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "收藏操作失败"),
    });
    const items = useMemo(() => query.data?.items || [], [query.data]);
    const copyItem = (item: InspirationItem) => {
        const run = () => mutation.mutateAsync(item.id);
        if (item.priceMicrocredits > 0 && !item.unlocked && !item.owner) {
            const price = formatCredits(item.priceMicrocredits);
            modal.confirm({
                title: `支付 ${price} 积分解锁？`,
                content: `确认后将从积分余额扣除 ${price} 积分，并同步记录到积分中心流水。解锁成功后会复制完整提示词和生成参数。`,
                okText: "确认支付并解锁",
                cancelText: "取消",
                onOk: async () => {
                    try {
                        const result = await unlockMutation.mutateAsync(item.id);
                        if (result.purchased) {
                            window.dispatchEvent(new CustomEvent("wallet:updated"));
                            message.success("积分支付成功，已永久解锁");
                        }
                        await queryClient.invalidateQueries({ queryKey: ["inspirations"] });
                        await run();
                    } catch (error) {
                        message.error(error instanceof Error ? error.message : "支付解锁失败");
                        throw error;
                    }
                },
            });
            return;
        }
        void run();
    };

    const filtersActive = Boolean(keyword || kind !== "all");
    return <WorkspacePage className="library-page inspiration-page" grid>
        <PageHeader icon="inspirations" title="灵感广场" description="发现社区作品，免费复刻或用积分解锁完整提示词与创作参数。" meta={<span className="text-xs text-foreground/45">{items.length} 份灵感</span>} />
        <div className="mt-1 flex flex-col border-b border-border/75 xl:flex-row xl:items-end xl:justify-between">
            <nav className="thin-scrollbar -mb-px flex min-w-0 overflow-x-auto" aria-label="灵感广场范围" role="tablist">
                {[{ value: "public", label: "灵感广场", icon: Sparkles }, { value: "favorites", label: "我的收藏", icon: Heart }].map((option) => {
                    const Icon = option.icon;
                    const active = scope === option.value;
                    return <button key={option.value} type="button" role="tab" aria-selected={active} className={`relative inline-flex h-12 shrink-0 items-center gap-2 border-b-2 px-3 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${active ? "border-foreground font-medium text-foreground" : "border-transparent text-foreground/48 hover:text-foreground/76"}`} onClick={() => setScope(option.value as "public" | "favorites")}><Icon className="size-4" />{option.label}</button>;
                })}
            </nav>
            <div className="library-toolbar-controls flex min-w-0 flex-wrap items-center gap-2 py-2.5 xl:flex-nowrap xl:justify-end">
                <Input allowClear className="w-full sm:!w-72" prefix={<Search className="size-4 text-foreground/38" />} value={keyword} placeholder="搜索作品或创作者" onChange={(event) => setKeyword(event.target.value)} />
                <Select className="w-[136px]" value={kind} onChange={setKind} options={[{ label: "全部类型", value: "all" }, { label: "图片", value: "image" }, { label: "视频", value: "video" }, { label: "短剧", value: "short-drama" }]} />
                {filtersActive ? <Button type="text" aria-label="重置筛选" icon={<RotateCcw className="size-4" />} onClick={() => { setKeyword(""); setKind("all"); }} /> : null}
            </div>
        </div>
        {query.isLoading ? <WorkspaceLoadingState label="正在收集灵感" detail="读取社区最新分享" /> : null}
        {query.isError ? <WorkspaceErrorState description={query.error instanceof Error ? query.error.message : "灵感加载失败"} onRetry={() => void query.refetch()} /> : null}
        {!query.isLoading && !query.isError && items.length ? <CollectionGrid className="inspiration-grid skill-library-grid py-6">{items.map((item) => <InspirationCard key={item.id} item={item} pending={(mutation.isPending || unlockMutation.isPending) && (mutation.variables === item.id || unlockMutation.variables === item.id)} favoritePending={favoriteMutation.isPending && favoriteMutation.variables?.id === item.id} onFavorite={() => favoriteMutation.mutate({ id: item.id, favorite: !item.favorite })} onCopy={() => copyItem(item)} />)}</CollectionGrid> : null}
        {!query.isLoading && !query.isError && !items.length ? <WorkspaceState compact className="min-h-[188px]" icon={scope === "favorites" ? "inspirations" : "empty"} title={filtersActive ? "没有找到匹配的灵感" : scope === "favorites" ? "还没有收藏灵感" : "灵感广场等待第一份作品"} description={filtersActive ? "换个关键词或类型试试。" : scope === "favorites" ? "在灵感广场点击爱心收藏，内容会显示在这里。" : "完成图片、视频或短剧创作后，就可以把作品分享到这里。"} /> : null}
    </WorkspacePage>;
}

export function InspirationCard({ item, pending, favoritePending = false, compact = false, onCopy, onFavorite }: { item: InspirationItem; pending: boolean; favoritePending?: boolean; compact?: boolean; onCopy: () => void; onFavorite?: () => void }) {
    const KindIcon = item.kind === "image" ? ImageIcon : item.kind === "video" ? Film : Sparkles;
    const paidLocked = item.priceMicrocredits > 0 && !item.unlocked && !item.owner;
    const priceLabel = item.owner ? "我的分享" : item.priceMicrocredits > 0 ? item.unlocked ? "已解锁" : `${formatCredits(item.priceMicrocredits)} 积分` : "免费";
    return <article className={`library-card library-card-surface inspiration-card${compact ? " is-compact" : ""}`}>
        <div className="inspiration-card-media">{item.previewUrl ? item.kind === "video" ? <video src={item.previewUrl} muted playsInline preload="metadata" /> : <img src={item.previewUrl} alt={item.title} loading="lazy" /> : <div className="inspiration-card-fallback"><KindIcon /></div>}<span className="inspiration-card-kind"><KindIcon />{item.kind === "image" ? "图片" : item.kind === "video" ? "视频" : "短剧"}</span><span className={`inspiration-card-price${item.priceMicrocredits > 0 ? " is-paid" : ""}`}>{item.priceMicrocredits > 0 ? <Coins /> : null}{priceLabel}</span></div>
        <div className="inspiration-card-body"><div className="inspiration-card-heading"><strong>{item.title}</strong></div>{compact ? null : <p>{item.description || item.promptPreview || (item.unlocked ? "完整创作参数已解锁" : "支付积分后查看完整提示词与参数")}</p>}<div className="inspiration-card-meta">{onFavorite ? <button type="button" disabled={favoritePending} className="inspiration-card-favorite" aria-label={item.favorite ? "取消收藏" : "收藏"} onClick={onFavorite}><Heart className={item.favorite ? "fill-current text-rose-500" : ""} /><span>{item.favoriteCount}</span></button> : <span />}<span className="inspiration-card-author">@{item.authorName}</span><span><Copy />{item.copyCount}</span></div><div className="inspiration-card-action"><Button type="primary" size="small" icon={pending ? <Spin size="small" /> : <WandSparkles className="size-3.5" />} loading={pending} onClick={onCopy}>{paidLocked ? "支付积分解锁并复制" : "一键复制到创作页"}</Button></div></div>
    </article>;
}
