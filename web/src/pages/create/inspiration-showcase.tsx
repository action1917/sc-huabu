import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App } from "antd";
import { ArrowRight } from "lucide-react";
import { Link, useNavigate } from "react-router";

import { saveInspirationReplica } from "@/lib/inspiration-replica";
import { listInspirations, replicateInspiration, setInspirationFavorite, unlockInspiration, type InspirationItem } from "@/services/api/inspirations";
import { InspirationCard } from "@/pages/inspirations";

export function InspirationShowcase() {
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const { message, modal } = App.useApp();
    const query = useQuery({ queryKey: ["inspirations", "showcase"], queryFn: () => listInspirations({ limit: 4 }) });
    const mutation = useMutation({
        mutationFn: replicateInspiration,
        onSuccess: ({ replica }) => { saveInspirationReplica(replica); message.success("灵感已复制"); navigate("/create?from=inspiration"); },
        onError: (error) => message.error(error instanceof Error ? error.message : "复制灵感失败"),
    });
    const unlockMutation = useMutation({ mutationFn: unlockInspiration });
    const favoriteMutation = useMutation({
        mutationFn: ({ id, favorite }: { id: string; favorite: boolean }) => setInspirationFavorite(id, favorite),
        onSuccess: () => void queryClient.invalidateQueries({ queryKey: ["inspirations"] }),
        onError: (error) => message.error(error instanceof Error ? error.message : "收藏操作失败"),
    });
    const copyItem = (item: InspirationItem) => {
        if (item.priceMicrocredits > 0 && !item.unlocked && !item.owner) {
            const price = item.priceMicrocredits / 1_000_000;
            modal.confirm({ title: `支付 ${price.toLocaleString("zh-CN")} 积分解锁？`, content: `确认后将扣除 ${price.toLocaleString("zh-CN")} 积分并写入积分中心流水，解锁成功后会自动填入完整提示词、模型和生成参数。`, okText: "确认支付并解锁", cancelText: "取消", onOk: async () => {
                try {
                    const result = await unlockMutation.mutateAsync(item.id);
                    if (result.purchased) window.dispatchEvent(new CustomEvent("wallet:updated"));
                    await queryClient.invalidateQueries({ queryKey: ["inspirations"] });
                    await mutation.mutateAsync(item.id);
                } catch (error) {
                    message.error(error instanceof Error ? error.message : "支付解锁失败");
                    throw error;
                }
            } });
            return;
        }
        mutation.mutate(item.id);
    };
    const items = query.data?.items || [];
    return <section className="creation-inspiration-showcase" aria-labelledby="creation-inspiration-title">
        <header><div><span>社区灵感</span><h2 id="creation-inspiration-title">从一个好作品开始</h2></div><Link to="/inspirations">查看更多<ArrowRight /></Link></header>
        {query.isLoading ? <div className="creation-inspiration-skeletons">{[0, 1, 2, 3].map((item) => <span key={item} />)}</div> : items.length ? <div className="creation-inspiration-grid">{items.map((item) => <InspirationCard key={item.id} item={item} compact pending={(mutation.isPending || unlockMutation.isPending) && (mutation.variables === item.id || unlockMutation.variables === item.id)} favoritePending={favoriteMutation.isPending && favoriteMutation.variables?.id === item.id} onFavorite={() => favoriteMutation.mutate({ id: item.id, favorite: !item.favorite })} onCopy={() => copyItem(item)} />)}</div> : <div className="creation-inspiration-empty"><strong>灵感广场等待第一份作品</strong><span>完成图片、视频或短剧后，点击“分享到灵感”即可发布。</span><Link to="/inspirations">进入灵感广场</Link></div>}
    </section>;
}
