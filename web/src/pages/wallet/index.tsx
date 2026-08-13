import { useEffect, useRef, useState, type ReactNode } from "react";
import { Alert, App, Button, Grid, Input, Modal, QRCode, Radio, Segmented, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import { motion, useReducedMotion } from "motion/react";
import { ArrowDownLeft, ArrowUpRight, CalendarCheck, Coins, CreditCard, RefreshCw, RotateCcw, ShieldCheck, SlidersHorizontal, Sparkles, TicketCheck } from "lucide-react";

import { formatCredits } from "@/constant/credits";
import { PaginationBar, TableSurface } from "@/components/layout/workspace-page";
import { WorkspaceState } from "@/components/layout/workspace-state";
import { WorkspaceSignalIcon } from "@/components/ui/aceternity/workspace-signal-icon";
import { CometCard } from "@/components/ui/aceternity/comet-card";
import { aceternityMotion } from "@/lib/aceternity-motion";
import { checkinCredits, createPaymentOrder, getPaymentCatalog, getPaymentOrder, getWallet, redeemCredits, type CreditLedgerEntry, type PaymentCatalog, type PaymentOrder, type WalletSummary } from "@/services/api/wallet";
import { modelDisplayName, useEffectiveConfig, type AiConfig } from "@/stores/use-config-store";

type LedgerFilter = "all" | "income" | "consume" | "refund";

const ledgerFilterOptions = [
    { label: "全部", value: "all" },
    { label: "充值与调整", value: "income" },
    { label: "消费与解锁", value: "consume" },
    { label: "退款", value: "refund" },
];

export default function WalletPage() {
    const { message } = App.useApp();
    const screens = Grid.useBreakpoint();
    const reducedMotion = useReducedMotion();
    const config = useEffectiveConfig();
    const [wallet, setWallet] = useState<WalletSummary | null>(null);
    const [code, setCode] = useState("");
    const [filter, setFilter] = useState<LedgerFilter>("all");
    const [loading, setLoading] = useState(false);
    const [redeeming, setRedeeming] = useState(false);
    const [checkingIn, setCheckingIn] = useState(false);
    const [paymentCatalog, setPaymentCatalog] = useState<PaymentCatalog | null>(null);
    const [paymentOpen, setPaymentOpen] = useState(false);
    const [paymentCreating, setPaymentCreating] = useState(false);
    const [selectedPackage, setSelectedPackage] = useState("");
    const [selectedProvider, setSelectedProvider] = useState<PaymentCatalog["providers"][number]["key"] | "">("");
    const [activePayment, setActivePayment] = useState<PaymentOrder | null>(null);
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(20);
    const requestSequence = useRef(0);

    const reload = async (targetPage = page, targetPageSize = pageSize) => {
        const sequence = ++requestSequence.current;
        setLoading(true);
        try {
            const nextWallet = await getWallet(targetPage, targetPageSize, filter);
            if (sequence === requestSequence.current) setWallet(nextWallet);
        } catch (error) {
            if (sequence === requestSequence.current) message.error(error instanceof Error ? error.message : "读取积分记录失败");
        } finally {
            if (sequence === requestSequence.current) setLoading(false);
        }
    };

    useEffect(() => {
        void reload(page, pageSize);
    }, [filter, page, pageSize]);

    useEffect(() => {
        void getPaymentCatalog()
            .then(({ catalog }) => {
                setPaymentCatalog(catalog);
                setSelectedPackage(catalog.packages[0]?.id || "");
                setSelectedProvider(catalog.providers[0]?.key || "");
            })
            .catch(() => setPaymentCatalog(null));
    }, []);

    useEffect(() => {
        const orderID = new URLSearchParams(window.location.search).get("paymentOrder") || activePayment?.id;
        if (!orderID) return;
        let stopped = false;
        let timer = 0;
        const check = async () => {
            try {
                const { order } = await getPaymentOrder(orderID);
                if (stopped) return;
                setActivePayment(order);
                if (order.status === "paid") {
                    window.clearInterval(timer);
                    await reload(page, pageSize);
                    window.dispatchEvent(new CustomEvent("wallet:updated"));
                    message.success("支付成功，积分已到账");
                    const nextURL = new URL(window.location.href);
                    nextURL.searchParams.delete("paymentOrder");
                    nextURL.searchParams.delete("paymentResult");
                    window.history.replaceState({}, "", nextURL.pathname + nextURL.search + nextURL.hash);
                } else if (order.status === "failed" || order.status === "canceled") {
                    window.clearInterval(timer);
                    message.error(order.error || "支付未完成");
                }
            } catch {
                window.clearInterval(timer);
            }
        };
        void check();
        timer = window.setInterval(() => void check(), 2500);
        return () => {
            stopped = true;
            window.clearInterval(timer);
        };
    }, [activePayment?.id]);

    const startPayment = async () => {
        if (!selectedPackage || !selectedProvider) return message.error("请选择充值套餐和支付方式");
        setPaymentCreating(true);
        try {
            const { order } = await createPaymentOrder({ packageId: selectedPackage, provider: selectedProvider });
            setActivePayment(order);
            if (!order.checkoutUrl) throw new Error("支付平台未返回收银台地址");
            if (order.checkoutMode === "redirect") window.location.assign(order.checkoutUrl);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "创建支付订单失败");
        } finally {
            setPaymentCreating(false);
        }
    };

    const redeem = async () => {
        const normalized = code.trim().toLowerCase();
        if (normalized.length !== 32) {
            message.error("请输入完整的 32 位兑换码");
            return;
        }
        setRedeeming(true);
        try {
            await redeemCredits(normalized);
            setCode("");
            setPage(1);
            await reload(1, pageSize);
            window.dispatchEvent(new CustomEvent("wallet:updated"));
            message.success("兑换成功，积分已到账");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "兑换失败");
        } finally {
            setRedeeming(false);
        }
    };

    const checkin = async () => {
        setCheckingIn(true);
        try {
            await checkinCredits();
            await reload(page, pageSize);
            window.dispatchEvent(new CustomEvent("wallet:updated"));
            message.success("签到成功，积分已到账");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "签到失败");
        } finally {
            setCheckingIn(false);
        }
    };

    const entries = wallet?.entries || [];
    const account = wallet?.account;
    const totalMicrocredits = (account?.availableMicrocredits || 0) + (account?.reservedMicrocredits || 0);

    const columns: ColumnsType<CreditLedgerEntry> = [
        { title: "发生时间", dataIndex: "createdAt", width: 180, render: formatTime },
        { title: "类型", dataIndex: "type", width: 120, render: (type) => <LedgerTypeTag type={type} /> },
        {
            title: "明细",
            width: 400,
            ellipsis: true,
            render: (_, entry) => (
                <div className="min-w-0 max-w-full overflow-hidden" title={[ledgerModelName(config, entry), [sceneLabel(entry.scene), entry.note].filter(Boolean).join(" · ")].filter(Boolean).join("\n")}>
                    <div className="truncate font-medium">{ledgerModelName(config, entry)}</div>
                    <div className="mt-1 truncate text-xs text-foreground/50">{[sceneLabel(entry.scene), entry.note].filter(Boolean).join(" · ") || "无补充说明"}</div>
                </div>
            ),
        },
        {
            title: "积分变化",
            dataIndex: "amountMicrocredits",
            width: 145,
            align: "right",
            render: (value: number) => <CreditDelta value={value} />,
        },
        { title: "变更后余额", dataIndex: "availableAfterMicrocredits", width: 145, align: "right", render: (value) => <span className="tabular-nums">{formatCredits(value)}</span> },
    ];

    return (
        <main className="app-user-content app-workspace-scroll library-page wallet-library-page relative h-full overflow-y-auto text-foreground">
            <div className="relative mx-auto max-w-[1440px] px-4 py-6 sm:px-6 lg:px-8">
                <motion.header
                    initial={reducedMotion ? false : { opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: aceternityMotion.duration.panel, ease: aceternityMotion.easing.enter }}
                    className="app-page-header flex flex-wrap items-start justify-between gap-4 pb-6"
                >
                    <div className="flex items-center gap-3">
                        <WorkspaceSignalIcon variant="wallet" />
                        <div>
                            <h1 className="text-[var(--fs-title)] font-semibold leading-7">积分中心</h1>
                            <p className="mt-1 text-xs leading-5 text-foreground/58">模型调用、灵感交易、冻结与退款都在同一条可追溯流水中。</p>
                        </div>
                    </div>
                    <div className="flex flex-wrap gap-2">
                        {paymentCatalog ? (
                            <Button icon={<CreditCard className="size-4" />} type="primary" onClick={() => setPaymentOpen(true)}>
                                {paymentCatalog.enabled && paymentCatalog.providers.length > 0 ? "在线充值" : "在线充值（待配置）"}
                            </Button>
                        ) : null}
                        <Button icon={<CalendarCheck className="size-4" />} type={wallet?.policy.checkedInToday ? "default" : "primary"} loading={checkingIn} disabled={wallet?.policy.checkedInToday} onClick={() => void checkin()}>
                            {wallet?.policy.checkedInToday ? "今日已签到" : `签到 +${formatCredits(wallet?.policy.checkinBonusMicrocredits || 0)}`}
                        </Button>
                        <Button icon={<RefreshCw className="size-4" />} loading={loading} onClick={() => void reload()}>
                            刷新余额
                        </Button>
                    </div>
                </motion.header>

                <section className="library-feature-grid mt-6 grid gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(340px,0.65fr)]">
                    <CometCard rotateDepth={2.2} translateDepth={2} glare={!reducedMotion} className="credit-balance-card overflow-hidden rounded-lg border">
                        <div className="flex min-h-[210px] flex-col justify-between p-5 sm:p-6">
                            <div className="flex items-start justify-between gap-4">
                                <div>
                                    <div className="flex items-center gap-2 text-xs font-medium opacity-60">
                                        <Coins className="size-4 text-amber-500" />
                                        可用创作积分
                                    </div>
                                    <div className="mt-4 text-5xl font-semibold tabular-nums">{formatCredits(account?.availableMicrocredits || 0, 6)}</div>
                                    <div className="mt-2 text-xs opacity-45">最近更新 {formatTime(account?.updatedAt)}</div>
                                </div>
                                <span className="inline-flex items-center gap-1.5 rounded-full border border-current/10 px-2.5 py-1 text-[var(--fs-label)] font-medium opacity-70">
                                    <ShieldCheck className="size-3.5" />
                                    账户正常
                                </span>
                            </div>
                            <div className="grid grid-cols-2 gap-3 border-t border-current/10 pt-4">
                                <BalanceMetric label="冻结积分" description="调用中或待核对" value={account?.reservedMicrocredits || 0} icon={<TicketCheck className="size-4" />} />
                                <BalanceMetric label="账户总额" description="可用与冻结合计" value={totalMicrocredits} icon={<Coins className="size-4" />} />
                            </div>
                        </div>
                    </CometCard>

                    <motion.div
                        initial={reducedMotion ? false : { opacity: 0, x: 12 }}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{ duration: aceternityMotion.duration.panel, ease: aceternityMotion.easing.enter }}
                        className="wallet-redeem-panel app-workspace-surface flex flex-col rounded-lg border p-5 backdrop-blur-xl sm:p-6"
                    >
                        <div className="flex items-start gap-3">
                            <span className="grid size-9 shrink-0 place-items-center rounded-xl border border-amber-400/25 bg-amber-400/10 text-amber-600 dark:text-amber-300">
                                <TicketCheck className="size-4" />
                            </span>
                            <div>
                                <h2 className="text-base font-semibold">兑换积分</h2>
                                <p className="mt-1 text-xs leading-5 text-foreground/55">输入管理员发放的 32 位兑换码。</p>
                            </div>
                        </div>
                        <label className="mt-6 block">
                            <span className="text-xs font-medium text-foreground/70">兑换码</span>
                            <Input
                                className="mt-2 font-mono"
                                size="large"
                                value={code}
                                maxLength={32}
                                spellCheck={false}
                                autoComplete="off"
                                onChange={(event) => setCode(event.target.value.replace(/[-\s]/g, ""))}
                                placeholder="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
                                onPressEnter={() => void redeem()}
                            />
                        </label>
                        <div className="mt-2 flex items-center justify-between text-xs text-foreground/45">
                            <span>兑换成功后立即到账</span>
                            <span className="tabular-nums">{code.length} / 32</span>
                        </div>
                        <Button className="mt-5" type="primary" size="large" block loading={redeeming} disabled={code.length !== 32} onClick={() => void redeem()}>
                            兑换积分
                        </Button>
                    </motion.div>
                </section>

                <PaymentModal
                    open={paymentOpen}
                    catalog={paymentCatalog}
                    packageID={selectedPackage}
                    provider={selectedProvider}
                    order={activePayment}
                    creating={paymentCreating}
                    onPackageChange={setSelectedPackage}
                    onProviderChange={setSelectedProvider}
                    onClose={() => {
                        setPaymentOpen(false);
                        if (activePayment?.status !== "pending") setActivePayment(null);
                    }}
                    onSubmit={() => void startPayment()}
                />

                <section className="wallet-ledger-panel app-workspace-surface mt-9 rounded-lg border p-4 backdrop-blur-xl sm:p-5">
                    <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
                        <div>
                            <h2 className="text-base font-semibold">积分流水</h2>
                            <p className="mt-1 text-xs text-foreground/55">当前展示最近 {wallet?.entries.length || 0} 条记录。</p>
                        </div>
                        <Segmented
                            block={!screens.sm}
                            value={filter}
                            options={ledgerFilterOptions}
                            onChange={(value) => {
                                setFilter(value as LedgerFilter);
                                setPage(1);
                            }}
                        />
                    </div>

                    {screens.md ? (
                        <TableSurface className="mt-0 rounded-xl border-border/70 bg-transparent">
                            <Table className="app-data-table wallet-ledger-table" rowKey="id" size="middle" loading={loading} columns={columns} dataSource={entries} pagination={false} tableLayout="fixed" scroll={{ x: 990 }} />
                        </TableSurface>
                    ) : (
                        <div className="overflow-hidden rounded-md border border-border/70 bg-background">
                            {entries.length ? (
                                entries.map((entry) => <LedgerMobileRow key={entry.id} config={config} entry={entry} />)
                            ) : (
                                <WorkspaceState compact icon="wallet" title="没有匹配的积分记录" description="切换流水类型，或完成一次生成后再回来查看。" />
                            )}
                        </div>
                    )}
                    <PaginationBar
                        current={page}
                        pageSize={pageSize}
                        total={wallet?.total || 0}
                        pageSizeOptions={[20, 50, 100]}
                        onChange={(nextPage, nextPageSize) => {
                            setPage(nextPageSize !== pageSize ? 1 : nextPage);
                            setPageSize(nextPageSize);
                        }}
                    />
                </section>
            </div>
        </main>
    );
}

function BalanceMetric({ label, description, value, icon }: { label: string; description: string; value: number; icon: ReactNode }) {
    return (
        <div className="flex min-w-0 items-center justify-between gap-3 rounded-xl bg-current/[0.055] px-3 py-2.5">
            <div className="min-w-0">
                <div className="text-[var(--fs-tiny)] opacity-48">{label}</div>
                <div className="mt-0.5 truncate text-base font-semibold tabular-nums">{formatCredits(value, 6)}</div>
                <div className="truncate text-[var(--fs-tiny)] opacity-35">{description}</div>
            </div>
            <span className="shrink-0 opacity-30">{icon}</span>
        </div>
    );
}

function LedgerMobileRow({ config, entry }: { config: AiConfig; entry: CreditLedgerEntry }) {
    const meta = ledgerTypeMeta(entry.type);
    return (
        <article className="flex items-start gap-3 border-b border-border px-4 py-4 last:border-b-0">
            <span className={`mt-0.5 grid size-8 shrink-0 place-items-center rounded-md ${meta.iconClass}`}>{meta.icon}</span>
            <div className="min-w-0 flex-1">
                <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                        <div className="truncate text-sm font-medium">{ledgerModelName(config, entry)}</div>
                        <div className="mt-1 text-xs text-foreground/45">{formatTime(entry.createdAt)}</div>
                    </div>
                    <CreditDelta value={entry.amountMicrocredits} />
                </div>
                <div className="mt-2 line-clamp-2 break-words text-xs leading-5 text-foreground/55">{[sceneLabel(entry.scene), entry.note].filter(Boolean).join(" · ") || meta.label}</div>
            </div>
        </article>
    );
}

function CreditDelta({ value }: { value: number }) {
    const colorClass = value > 0 ? "text-emerald-600 dark:text-emerald-400" : value < 0 ? "text-rose-600 dark:text-rose-400" : "text-foreground/60";
    return (
        <span className={`shrink-0 font-medium tabular-nums ${colorClass}`}>
            {value > 0 ? "+" : ""}
            {formatCredits(value, 6)}
        </span>
    );
}

function LedgerTypeTag({ type }: { type: CreditLedgerEntry["type"] }) {
    const meta = ledgerTypeMeta(type);
    return (
        <Tag variant="filled" color={meta.tagColor}>
            {meta.label}
        </Tag>
    );
}

function ledgerTypeMeta(type: CreditLedgerEntry["type"]) {
    const values = {
        redeem: { label: "兑换充值", tagColor: "success", icon: <ArrowDownLeft className="size-4" />, iconClass: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-300" },
        admin_grant: { label: "管理员充值", tagColor: "blue", icon: <ArrowDownLeft className="size-4" />, iconClass: "bg-sky-500/10 text-sky-600 dark:text-sky-300" },
        consume: { label: "模型消费", tagColor: "error", icon: <Sparkles className="size-4" />, iconClass: "bg-rose-500/10 text-rose-600 dark:text-rose-300" },
        reserve: { label: "积分冻结", tagColor: "warning", icon: <ArrowUpRight className="size-4" />, iconClass: "bg-amber-500/10 text-amber-600 dark:text-amber-300" },
        refund: { label: "消费退款", tagColor: "warning", icon: <RotateCcw className="size-4" />, iconClass: "bg-amber-500/10 text-amber-600 dark:text-amber-300" },
        admin_adjustment: { label: "管理员调账", tagColor: "default", icon: <SlidersHorizontal className="size-4" />, iconClass: "bg-muted text-foreground/60" },
        signup_bonus: { label: "注册奖励", tagColor: "gold", icon: <Sparkles className="size-4" />, iconClass: "bg-amber-500/10 text-amber-600 dark:text-amber-300" },
        checkin_bonus: { label: "签到奖励", tagColor: "cyan", icon: <CalendarCheck className="size-4" />, iconClass: "bg-cyan-500/10 text-cyan-600 dark:text-cyan-300" },
        payment: { label: "在线充值", tagColor: "success", icon: <CreditCard className="size-4" />, iconClass: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-300" },
        inspiration_purchase: { label: "购买灵感", tagColor: "error", icon: <Coins className="size-4" />, iconClass: "bg-rose-500/10 text-rose-600 dark:text-rose-300" },
        inspiration_sale: { label: "灵感售出", tagColor: "success", icon: <Coins className="size-4" />, iconClass: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-300" },
    } as const;
    return values[type] || { label: "其他积分变动", tagColor: "default", icon: <ArrowUpRight className="size-4" />, iconClass: "bg-muted text-foreground/60" };
}

function ledgerTitle(entry: CreditLedgerEntry) {
    if (entry.type === "redeem") return "兑换码充值";
    if (entry.type === "refund") return "模型消费退款";
    if (entry.type === "consume") return "模型调用";
    if (entry.type === "signup_bonus") return "新用户注册奖励";
    if (entry.type === "checkin_bonus") return "每日签到奖励";
    if (entry.type === "payment") return "在线支付充值";
    if (entry.type === "inspiration_purchase") return "购买灵感";
    if (entry.type === "inspiration_sale") return "灵感售出";
    return entry.note || "积分调整";
}

function PaymentModal({
    open,
    catalog,
    packageID,
    provider,
    order,
    creating,
    onPackageChange,
    onProviderChange,
    onClose,
    onSubmit,
}: {
    open: boolean;
    catalog: PaymentCatalog | null;
    packageID: string;
    provider: PaymentCatalog["providers"][number]["key"] | "";
    order: PaymentOrder | null;
    creating: boolean;
    onPackageChange: (value: string) => void;
    onProviderChange: (value: PaymentCatalog["providers"][number]["key"]) => void;
    onClose: () => void;
    onSubmit: () => void;
}) {
    const selectedProvider = catalog?.providers.find((item) => item.key === provider);
    const qrcodePending = order?.checkoutMode === "qrcode" && order.status === "pending" && order.checkoutUrl;
    const paymentAvailable = Boolean(catalog?.enabled && catalog.providers.length > 0 && catalog.packages.length > 0);
    return (
        <Modal
            title="在线充值"
            open={open}
            onCancel={onClose}
            okText={qrcodePending ? "等待支付" : paymentAvailable ? "前往支付" : "暂未开放"}
            okButtonProps={{ disabled: Boolean(qrcodePending) || !paymentAvailable || !packageID || !provider }}
            confirmLoading={creating}
            onOk={onSubmit}
        >
            {qrcodePending ? (
                <div className="flex flex-col items-center py-5">
                    <QRCode value={order.checkoutUrl!} size={220} />
                    <div className="mt-4 text-sm font-medium">请使用微信扫描二维码完成支付</div>
                    <div className="mt-1 text-xs text-foreground/50">支付成功后积分会自动到账，请勿重复付款。</div>
                </div>
            ) : (
                <div className="space-y-5 py-3">
                    {!paymentAvailable ? (
                        <Alert
                            showIcon
                            type="info"
                            message="在线充值渠道待配置"
                            description="管理员正在配置支付宝、微信、Stripe 或 PayPal。配置完成后，本页面会自动显示充值套餐和支付方式。"
                        />
                    ) : null}
                    <div>
                        <div className="mb-2 text-xs font-medium text-foreground/65">选择充值套餐</div>
                        <Radio.Group className="grid w-full grid-cols-1 gap-2 sm:grid-cols-2" value={packageID} onChange={(event) => onPackageChange(event.target.value)}>
                            {catalog?.packages.map((item) => {
                                const amount = selectedProvider?.currency === "CNY" ? `¥${(item.cnyAmountMinor / 100).toFixed(2)}` : `$${(item.usdAmountMinor / 100).toFixed(2)}`;
                                return (
                                    <Radio.Button key={item.id} value={item.id} className="!h-auto !rounded-md !border !px-3 !py-2">
                                        <span className="block font-medium">{item.name}</span>
                                        <span className="text-xs text-foreground/55">
                                            {formatCredits(item.creditMicrocredits)} 积分 · {amount}
                                        </span>
                                    </Radio.Button>
                                );
                            })}
                        </Radio.Group>
                    </div>
                    <div>
                        <div className="mb-2 text-xs font-medium text-foreground/65">选择支付方式</div>
                        <Radio.Group value={provider} onChange={(event) => onProviderChange(event.target.value)}>
                            {catalog?.providers.map((item) => (
                                <Radio.Button key={item.key} value={item.key}>
                                    {item.name}
                                </Radio.Button>
                            ))}
                        </Radio.Group>
                    </div>
                    <div className="rounded-md bg-muted/50 px-3 py-2 text-xs leading-5 text-foreground/55">付款结果由服务端验签后入账。支付页面显示成功但积分尚未到账时，请返回本页等待几秒。</div>
                </div>
            )}
        </Modal>
    );
}

function ledgerModelName(config: AiConfig, entry: CreditLedgerEntry) {
    return entry.model ? modelDisplayName(config, entry.model) : ledgerTitle(entry);
}

function sceneLabel(scene?: string) {
    const labels: Record<string, string> = { image: "图片生成", text: "文本生成", video: "视频生成", audio: "音频生成", storyboard: "分镜生成", inspiration: "灵感广场" };
    return scene ? labels[scene] || "其他场景" : "";
}

function formatTime(value?: string) {
    return value ? new Date(value).toLocaleString("zh-CN", { hour12: false }) : "--";
}
