import { CheckCheck, Copy, Download, FileUp, FolderOpen, MoreHorizontal, PencilLine, Plus, RotateCcw, Search, Sparkles, Trash2, Upload } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { App, Button, Drawer, Dropdown, Form, Input, Modal, Select, Space, Tag, Tooltip, Typography } from "antd";

import { CollectionGrid, PageHeader, PaginationBar, WorkspacePage } from "@/components/layout/workspace-page";
import { WorkspaceState } from "@/components/layout/workspace-state";
import { AssetMediaPreview } from "@/components/asset-media-preview";
import { AssetLibraryCard, AssetLibraryCardMedia } from "@/components/assets/asset-library-card";
import { saveAs } from "file-saver";

import { useCopyText } from "@/hooks/use-copy-text";
import { resourceStorageLabel, resourceStorageLocation, resourceStorageTitle } from "@/lib/canvas/resource-storage-status";
import { formatBytes, readFileAsDataUrl } from "@/lib/image-utils";
import { getAssetLibraryScope, type AssetLibraryScope } from "@/lib/asset-library-scope";
import { uploadImage } from "@/services/image-storage";
import { uploadMediaFile } from "@/services/file-storage";
import { useAssetStore, type Asset, type AssetCategory, type AssetKind, type ImageAsset } from "@/stores/use-asset-store";
import { exportAssets, readAssetPackage } from "./asset-transfer";
import { deleteAssetWithRemoteSync } from "@/services/user-data-sync";

type LibraryAsset = Exclude<Asset, { kind: "entity" }>;

type AssetFormValues = {
    kind: AssetKind;
    category: AssetCategory;
    title: string;
    coverUrl: string;
    tags: string[];
    source?: string;
    note?: string;
    content?: string;
};

type ImageDraft = ImageAsset["data"] | null;

const kindOptions = [
    { label: "全部", value: "all" },
    { label: "文本", value: "text" },
    { label: "图片", value: "image" },
    { label: "视频", value: "video" },
    { label: "音频", value: "audio" },
    { label: "3D 模型", value: "model" },
];

const categoryOptions = [
    { label: "全部分类", value: "all" },
    { label: "角色", value: "character" },
    { label: "场景", value: "environment" },
    { label: "服饰", value: "wardrobe" },
    { label: "道具", value: "prop" },
    { label: "武器", value: "weapon" },
    { label: "画风", value: "style" },
    { label: "其他", value: "other" },
];

const scopeOptions = [
    { label: "作品库", value: "works", icon: Sparkles },
    { label: "素材库", value: "materials", icon: FolderOpen },
] as const;

export default function AssetsPage() {
    const { message } = App.useApp();
    const copyText = useCopyText();
    const [form] = Form.useForm<AssetFormValues>();
    const coverInputRef = useRef<HTMLInputElement>(null);
    const imageInputRef = useRef<HTMLInputElement>(null);
    const assetInputRef = useRef<HTMLInputElement>(null);
    const modelInputRef = useRef<HTMLInputElement>(null);
    const assets = useAssetStore((state) => state.assets);
    const addAsset = useAssetStore((state) => state.addAsset);
    const updateAsset = useAssetStore((state) => state.updateAsset);
    const [keyword, setKeyword] = useState("");
    const [scope, setScope] = useState<AssetLibraryScope>("works");
    const [kindFilter, setKindFilter] = useState<AssetKind | "all">("all");
    const [categoryFilter, setCategoryFilter] = useState<AssetCategory | "all">("all");
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(20);
    const [editingAsset, setEditingAsset] = useState<LibraryAsset | null>(null);
    const [isAssetOpen, setIsAssetOpen] = useState(false);
    const [previewAsset, setPreviewAsset] = useState<LibraryAsset | null>(null);
    const [deletingAsset, setDeletingAsset] = useState<LibraryAsset | null>(null);
    const [selectedIds, setSelectedIds] = useState<string[]>([]);
    const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
    const [formKind, setFormKind] = useState<AssetKind>("text");
    const [imageDraft, setImageDraft] = useState<ImageDraft>(null);
    const coverUrl = Form.useWatch("coverUrl", form) || "";
    const title = Form.useWatch("title", form) || "";
    const tags = Form.useWatch("tags", form) || [];
    const content = Form.useWatch("content", form) || "";
    const validAssets = useMemo(() => assets.filter((asset): asset is LibraryAsset => asset.kind !== "entity"), [assets]);
    const scopedAssets = useMemo(() => validAssets.filter((asset) => getAssetLibraryScope(asset) === scope), [scope, validAssets]);
    const selectedAssets = useMemo(() => scopedAssets.filter((asset) => selectedIds.includes(asset.id)), [selectedIds, scopedAssets]);
    const canCreateAsset = scope === "materials" && !keyword.trim() && kindFilter === "all" && categoryFilter === "all";

    const filteredAssets = useMemo(() => {
        const query = keyword.trim().toLowerCase();
        return scopedAssets.filter((asset) => {
            if (kindFilter !== "all" && asset.kind !== kindFilter) return false;
            if (categoryFilter !== "all" && (asset.category || "other") !== categoryFilter) return false;
            if (!query) return true;
            return assetSearchText(asset).includes(query);
        });
    }, [scopedAssets, keyword, kindFilter, categoryFilter]);
    const filteredAssetIds = useMemo(() => filteredAssets.map((asset) => asset.id), [filteredAssets]);
    const allFilteredSelected = filteredAssetIds.length > 0 && filteredAssetIds.every((id) => selectedIds.includes(id));

    const visibleAssets = useMemo(() => {
        const start = (page - 1) * pageSize;
        return filteredAssets.slice(start, start + pageSize);
    }, [filteredAssets, page, pageSize]);

    useEffect(() => {
        const maxPage = Math.max(1, Math.ceil(filteredAssets.length / pageSize));
        setPage((value) => Math.min(value, maxPage));
    }, [filteredAssets.length, pageSize]);

    useEffect(() => {
        const existingIds = new Set(scopedAssets.map((asset) => asset.id));
        setSelectedIds((current) => current.filter((id) => existingIds.has(id)));
    }, [scopedAssets]);

    const openCreate = () => {
        setEditingAsset(null);
        setImageDraft(null);
        setFormKind("text");
        form.setFieldsValue({ kind: "text", category: "other", title: "", coverUrl: "", tags: [], source: "手动添加", note: "", content: "" });
        setIsAssetOpen(true);
    };

    const openEdit = (asset: LibraryAsset) => {
        setEditingAsset(asset);
        setFormKind(asset.kind);
        setImageDraft(asset.kind === "image" ? asset.data : null);
        form.setFieldsValue({
            kind: asset.kind,
            category: asset.category || "other",
            title: asset.title,
            coverUrl: asset.coverUrl,
            tags: asset.tags || [],
            source: asset.source,
            note: asset.note,
            content: asset.kind === "text" ? asset.data.content : "",
        });
        setIsAssetOpen(true);
    };

    const saveAsset = async () => {
        const values = await form.validateFields();
        const base = {
            title: values.title.trim(),
            category: values.category,
            status: editingAsset?.status || "confirmed" as const,
            primaryVersionId: editingAsset?.primaryVersionId,
            coverUrl: values.coverUrl?.trim() || (values.kind === "image" && imageDraft ? imageDraft.dataUrl : ""),
            tags: values.tags || [],
            source: values.source?.trim(),
            note: values.note?.trim(),
            metadata: editingAsset?.metadata || { source: "manual" },
        };

        if (values.kind === "text") {
            const asset = { ...base, kind: "text" as const, data: { content: (values.content || "").trim() } };
            editingAsset ? updateAsset(editingAsset.id, asset) : addAsset(asset);
        } else {
            if (!imageDraft) {
                message.error("请选择图片文件");
                return;
            }
            const asset = { ...base, kind: "image" as const, data: imageDraft };
            editingAsset ? updateAsset(editingAsset.id, asset) : addAsset(asset);
        }

        message.success(editingAsset ? "素材已更新" : "素材已保存");
        setIsAssetOpen(false);
    };

    const readCoverFile = async (file?: File) => {
        if (!file) return;
        const dataUrl = await readFileAsDataUrl(file);
        form.setFieldValue("coverUrl", dataUrl);
    };

    const readImageFile = async (file?: File) => {
        if (!file || !file.type.startsWith("image/")) return;
        const image = await uploadImage(file);
        const draft = { dataUrl: image.url, storageKey: image.storageKey, width: image.width, height: image.height, bytes: image.bytes, mimeType: image.mimeType };
        setImageDraft(draft);
        if (!form.getFieldValue("coverUrl")) form.setFieldValue("coverUrl", draft.dataUrl);
        if (!form.getFieldValue("title")) form.setFieldValue("title", file.name);
    };

    const readModelFile = async (file?: File) => {
        if (!file || !/\.(glb|gltf)$/i.test(file.name)) return;
        const uploaded = await uploadMediaFile(file, "model");
        addAsset({ kind: "model", title: file.name.replace(/\.(glb|gltf)$/i, ""), coverUrl: "", tags: ["3D模型"], source: "手动上传", data: { url: uploaded.url, storageKey: uploaded.storageKey, bytes: uploaded.bytes, mimeType: uploaded.mimeType, fileName: file.name }, metadata: { source: "manual" } });
        message.success("3D 模型已保存");
    };

    const copyAssetText = async (asset: LibraryAsset) => {
        if (asset.kind !== "text") return;
        copyText(asset.data.content, "文本已复制");
    };

    const downloadImage = (asset: LibraryAsset) => {
        if (asset.kind !== "image" && asset.kind !== "video" && asset.kind !== "audio" && asset.kind !== "model") return;
        const url = asset.kind === "image" ? asset.data.dataUrl : asset.data.url;
        const extension = asset.kind === "model" ? asset.data.fileName.split(".").pop() || "glb" : asset.data.mimeType.split("/")[1] || "png";
        saveAs(url, `${asset.title || "asset"}.${extension}`);
    };

    const exportAllAssets = async () => {
        if (!scopedAssets.length) {
            message.warning(`暂无${scope === "works" ? "作品" : "素材"}可导出`);
            return;
        }
        await exportAssets(scopedAssets);
    };

    const importAssetZip = async (file?: File) => {
        if (!file) return;
        try {
            const importedAssets = await readAssetPackage(file);
            importedAssets.forEach((asset) => {
                const payload = { ...asset } as Record<string, unknown>;
                delete payload.id;
                delete payload.createdAt;
                delete payload.updatedAt;
                const imported = payload as Parameters<typeof addAsset>[0];
                addAsset({ ...imported, source: imported.source || "素材包导入", metadata: { ...(imported.metadata || {}), source: "asset-package-import" } });
            });
            message.success(`已导入 ${importedAssets.length} 个素材`);
        } catch {
            message.error("导入失败，请选择有效的素材压缩包");
        } finally {
            if (assetInputRef.current) assetInputRef.current.value = "";
        }
    };

    const confirmDelete = async () => {
        if (!deletingAsset) return;
        try {
            await deleteAssetWithRemoteSync(deletingAsset.id);
            message.success("素材已删除");
            setDeletingAsset(null);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "素材删除失败");
        }
    };

    const exportSelectedAssets = async () => {
        if (!selectedAssets.length) return;
        await exportAssets(selectedAssets);
    };

    const confirmBatchDelete = async () => {
        if (!selectedAssets.length) return;
        try {
            for (const asset of selectedAssets) await deleteAssetWithRemoteSync(asset.id);
            message.success(`已删除 ${selectedAssets.length} 个素材`);
            setSelectedIds([]);
            setBatchDeleteOpen(false);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "批量删除失败");
        }
    };

    return (
        <>
            <WorkspacePage grid className="library-page assets-library-page">
                <PageHeader
                    icon="assets"
                    title="资产"
                    description="作品与创作素材分区管理，生成结果自动进入作品库，上传和新建内容进入素材库。"
                    meta={<span className="text-xs text-foreground/45">{filteredAssets.length} 个{scope === "works" ? "作品" : "素材"}</span>}
                    actions={(
                        <>
                            <Button icon={<Download className="size-4" />} onClick={() => void exportAllAssets()}>导出当前</Button>
                            {scope === "materials" ? <Dropdown trigger={["click"]} menu={{ items: [{ key: "package", icon: <FileUp className="size-4" />, label: "导入素材包", onClick: () => assetInputRef.current?.click() }, { key: "model", icon: <Upload className="size-4" />, label: "上传 3D 模型", onClick: () => modelInputRef.current?.click() }] }}><Button icon={<FileUp className="size-4" />}>导入</Button></Dropdown> : null}
                        </>
                    )}
                />
                <div className="mt-1 flex flex-col border-b border-border/75 xl:flex-row xl:items-end xl:justify-between">
                    <nav className="thin-scrollbar -mb-px flex min-w-0 overflow-x-auto" aria-label="资产分区" role="tablist">
                        {scopeOptions.map((option) => {
                            const Icon = option.icon;
                            const active = scope === option.value;
                            return (
                                <button
                                    key={option.value}
                                    type="button"
                                    role="tab"
                                    aria-selected={active}
                                    className={`relative inline-flex h-12 shrink-0 items-center border-b-2 px-3 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${active ? "border-foreground font-medium text-foreground" : "border-transparent text-foreground/48 hover:text-foreground/76"}`}
                                    onClick={() => { setScope(option.value); setSelectedIds([]); setPage(1); }}
                                >
                                    <Icon className="mr-2 size-4" />{option.label}
                                </button>
                            );
                        })}
                    </nav>

                    <div className="library-toolbar-controls flex min-w-0 flex-wrap items-center gap-2 py-2.5 xl:flex-nowrap xl:justify-end">
                        <Input allowClear className="w-full sm:!w-64" prefix={<Search className="size-4 text-foreground/38" />} value={keyword} placeholder={`搜索${scope === "works" ? "作品" : "素材"}或来源`} onChange={(event) => { setPage(1); setKeyword(event.target.value); }} />
                        <Select className="w-[124px]" aria-label="资产类型" value={kindFilter} options={kindOptions.map((option) => ({ ...option, label: option.value === "all" ? "全部类型" : option.label }))} onChange={(value) => { setKindFilter(value as AssetKind | "all"); setPage(1); }} />
                        <Select className="w-[136px]" aria-label="业务分类" value={categoryFilter} options={categoryOptions} onChange={(value) => { setCategoryFilter(value as AssetCategory | "all"); setPage(1); }} />
                        {keyword || kindFilter !== "all" || categoryFilter !== "all" ? (
                            <Tooltip title="重置筛选">
                                <Button type="text" aria-label="重置筛选" icon={<RotateCcw className="size-4" />} onClick={() => { setKeyword(""); setKindFilter("all"); setCategoryFilter("all"); setPage(1); }} />
                            </Tooltip>
                        ) : null}
                    </div>
                </div>

                <section className="min-w-0">
                    {selectedAssets.length ? (
                            <div className="mt-3 flex min-h-10 flex-wrap items-center gap-2 border-y border-border/75 py-2 text-xs">
                                <strong className="mr-auto font-medium">已选择 {selectedAssets.length} 个{scope === "works" ? "作品" : "素材"}</strong>
                                <Button size="small" icon={<CheckCheck className="size-3.5" />} disabled={allFilteredSelected} onClick={() => setSelectedIds((current) => Array.from(new Set([...current, ...filteredAssetIds])))}>全选</Button>
                                <Button size="small" onClick={() => setSelectedIds([])}>取消选择</Button>
                                <Button size="small" icon={<Download className="size-3.5" />} onClick={() => void exportSelectedAssets()}>导出</Button>
                                <Button size="small" danger icon={<Trash2 className="size-3.5" />} onClick={() => setBatchDeleteOpen(true)}>删除</Button>
                            </div>
                    ) : null}
                    <CollectionGrid className="library-grid assets-library-grid">
                            {canCreateAsset ? <button type="button" className="library-create-card" onClick={openCreate}>
                                <span className="library-create-cover"><Plus className="size-8" /></span>
                                <span className="library-create-title">新增素材</span>
                                <span className="library-create-meta">文本、图片、音视频或模型</span>
                            </button> : null}
                            {visibleAssets.map((asset) => (
                                <AssetCard key={asset.id} asset={asset} selected={selectedIds.includes(asset.id)} onSelect={(selected) => setSelectedIds((current) => selected ? [...new Set([...current, asset.id])] : current.filter((id) => id !== asset.id))} onOpen={() => setPreviewAsset(asset)} onEdit={() => openEdit(asset)} onCopy={copyAssetText} onDownload={downloadImage} onDelete={() => setDeletingAsset(asset)} />
                            ))}
                    </CollectionGrid>

                    {!visibleAssets.length && (scopedAssets.length > 0 || !canCreateAsset) ? <WorkspaceState icon="assets" compact title={scopedAssets.length ? `没有匹配的${scope === "works" ? "作品" : "素材"}` : scope === "works" ? "作品库还是空的" : "素材库还是空的"} description={scopedAssets.length ? "调整关键词或筛选条件后再试。" : scope === "works" ? "完成图片、视频、音频等生成任务后，结果会自动收录到这里。" : "上传文件、导入素材包或新建素材后，会统一收录到这里。"} action={scope === "materials" && !scopedAssets.length ? <Button type="primary" icon={<Plus className="size-4" />} onClick={openCreate}>新增素材</Button> : undefined} /> : null}

                    <PaginationBar current={page} pageSize={pageSize} total={filteredAssets.length} pageSizeOptions={[20, 40, 80]} onChange={(nextPage, nextPageSize) => { setPage(nextPageSize !== pageSize ? 1 : nextPage); setPageSize(nextPageSize); }} />
                </section>
            </WorkspacePage>

            <Modal className="library-modal" title={editingAsset ? "编辑素材" : "新增素材"} open={isAssetOpen} width={980} onCancel={() => setIsAssetOpen(false)} onOk={() => void saveAsset()} okText="保存" cancelText="取消" destroyOnHidden>
                <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_280px]">
                    <Form form={form} layout="vertical" requiredMark={false} initialValues={{ kind: "text", category: "other", tags: [] }}>
                        <Form.Item name="kind" label="类型">
                            <Select
                                options={[
                                    { label: "文本", value: "text" },
                                    { label: "图片", value: "image" },
                                ]}
                                onChange={(value) => setFormKind(value)}
                            />
                        </Form.Item>
                        <Form.Item name="category" label="业务分类">
                            <Select options={categoryOptions.slice(1)} />
                        </Form.Item>
                        <Form.Item name="title" label="标题" rules={[{ required: true, message: "请输入标题" }]}>
                                <Input placeholder="给素材起一个容易检索的名字" />
                        </Form.Item>
                        <Form.Item name="coverUrl" label="封面 URL">
                            <Space.Compact className="w-full">
                                <Input placeholder="可粘贴图片 URL，也可以上传本地封面" />
                                <Button icon={<Upload className="size-3.5" />} onClick={() => coverInputRef.current?.click()}>
                                    上传
                                </Button>
                            </Space.Compact>
                        </Form.Item>
                        <Form.Item name="tags" label="标签">
                            <Select mode="tags" tokenSeparators={[",", "，"]} placeholder="输入标签后回车" />
                        </Form.Item>
                        <div className="grid gap-4 sm:grid-cols-2">
                            <Form.Item name="source" label="来源">
                                <Input placeholder="手动添加 / 画布 / 任务中心" />
                            </Form.Item>
                            <Form.Item name="note" label="备注">
                                <Input placeholder="可选" />
                            </Form.Item>
                        </div>
                        {formKind === "text" ? (
                            <Form.Item name="content" label="文本内容" rules={[{ required: true, message: "请输入文本内容" }]}>
                                <Input.TextArea rows={8} placeholder="保存提示词、说明文案、参考描述等文本素材" />
                            </Form.Item>
                        ) : (
                            <Form.Item label="图片内容" required>
                                <div className="rounded-lg border border-dashed border-stone-300 p-4 dark:border-stone-700">
                                    <Button icon={<Upload className="size-4" />} onClick={() => imageInputRef.current?.click()}>
                                        选择图片文件
                                    </Button>
                                    {imageDraft ? (
                                        <Typography.Text type="secondary" className="ml-3 text-xs" title={resourceStorageTitle(imageDraft.storageKey)}>
                                            {imageDraft.width}x{imageDraft.height} · {formatBytes(imageDraft.bytes)} · {resourceStorageLabel(imageDraft.storageKey)}
                                        </Typography.Text>
                                    ) : (
                                        <Typography.Text type="secondary" className="ml-3 text-xs">
                                            未选择图片
                                        </Typography.Text>
                                    )}
                                </div>
                            </Form.Item>
                        )}
                    </Form>
                    <div className="lg:border-l lg:border-border lg:pl-4">
                        <Typography.Text strong className="text-xs">预览</Typography.Text>
                        <div className="mt-2 overflow-hidden rounded-md bg-stone-100 dark:bg-stone-900">
                            {coverUrl || imageDraft?.dataUrl ? (
                                <img src={coverUrl || imageDraft?.dataUrl} alt="" loading="lazy" decoding="async" className="aspect-[4/3] w-full object-cover" />
                            ) : (
                                <div className="flex aspect-[4/3] items-center justify-center bg-stone-100 p-5 text-center text-sm text-stone-500 dark:bg-stone-900">{content || "暂无封面"}</div>
                            )}
                            <div className="border-x border-b border-border bg-background p-3">
                                <Typography.Text strong ellipsis className="block">
                                    {title || "未命名素材"}
                                </Typography.Text>
                                <div className="mt-2 flex flex-wrap gap-1.5">
                                    {tags.length ? (
                                        tags.map((tag) => (
                                            <Tag key={tag} className="m-0">
                                                {tag}
                                            </Tag>
                                        ))
                                    ) : (
                                        <Tag className="m-0">未打标签</Tag>
                                    )}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
                <input
                    ref={coverInputRef}
                    type="file"
                    accept="image/*"
                    className="hidden"
                    onChange={(event) => {
                        void readCoverFile(event.target.files?.[0]);
                        event.target.value = "";
                    }}
                />
                <input
                    ref={imageInputRef}
                    type="file"
                    accept="image/*"
                    className="hidden"
                    onChange={(event) => {
                        void readImageFile(event.target.files?.[0]);
                        event.target.value = "";
                    }}
                />
            </Modal>

            <AssetDrawer asset={previewAsset} onClose={() => setPreviewAsset(null)} onCopy={copyAssetText} onDownload={downloadImage} />

            <input ref={assetInputRef} type="file" accept="application/zip,.zip" className="hidden" onChange={(event) => void importAssetZip(event.target.files?.[0])} />
            <input ref={modelInputRef} type="file" accept=".glb,.gltf,model/gltf-binary,model/gltf+json" className="hidden" onChange={(event) => { void readModelFile(event.target.files?.[0]); event.currentTarget.value = ""; }} />

            <Modal className="library-modal library-confirm-modal" title="删除素材" open={Boolean(deletingAsset)} onCancel={() => setDeletingAsset(null)} onOk={() => void confirmDelete()} okText="删除" okButtonProps={{ danger: true }} cancelText="取消">
                确定删除「{deletingAsset?.title}」吗？删除后会从我的素材中移除。
            </Modal>
            <Modal className="library-modal library-confirm-modal" title="批量删除素材" open={batchDeleteOpen} onCancel={() => setBatchDeleteOpen(false)} onOk={() => void confirmBatchDelete()} okText="删除" okButtonProps={{ danger: true }} cancelText="取消">
                确定删除已选择的 {selectedAssets.length} 个素材吗？此操作会同步移除远端记录。
            </Modal>
        </>
    );
}

function AssetCard({ asset, selected, onSelect, onOpen, onEdit, onCopy, onDownload, onDelete }: { asset: LibraryAsset; selected: boolean; onSelect: (selected: boolean) => void; onOpen: () => void; onEdit: () => void; onCopy: (asset: LibraryAsset) => void; onDownload: (asset: LibraryAsset) => void; onDelete: () => void }) {
    const summary = assetSummary(asset);
    return (
        <AssetLibraryCard selected={selected}>
            <AssetLibraryCardMedia className="relative overflow-hidden bg-foreground/[.045]">
                <button type="button" className="block w-full text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[var(--workspace-accent)]" onClick={onOpen} aria-label={`查看素材：${asset.title}`}>
                    <AssetMediaPreview asset={asset} alt={asset.title} className="aspect-[4/3] w-full bg-black object-cover" fallback={<div className="flex aspect-[4/3] items-center justify-center bg-foreground/[.035] p-4 text-center text-xs leading-5 text-foreground/55">{asset.kind === "text" ? asset.data.content : "暂无封面"}</div>} />
                </button>
                <input type="checkbox" checked={selected} onClick={(event) => event.stopPropagation()} onChange={(event) => onSelect(event.target.checked)} className={`absolute left-2 top-2 size-4 accent-[var(--workspace-accent)] drop-shadow-md transition-opacity ${selected ? "opacity-100" : "opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"}`} aria-label={`选择 ${asset.title}`} />
                <Dropdown
                    trigger={["click"]}
                    menu={{ items: [
                        ...(asset.kind !== "video" && asset.kind !== "audio" && asset.kind !== "model" ? [{ key: "edit", icon: <PencilLine className="size-3.5" />, label: "编辑", onClick: onEdit }] : []),
                        ...(asset.kind === "text" ? [{ key: "copy", icon: <Copy className="size-3.5" />, label: "复制文本", onClick: () => void onCopy(asset) }] : []),
                        ...(asset.kind === "image" || asset.kind === "video" || asset.kind === "audio" || asset.kind === "model" ? [{ key: "download", icon: <Download className="size-3.5" />, label: "下载", onClick: () => onDownload(asset) }] : []),
                        { type: "divider" as const },
                        { key: "delete", danger: true, icon: <Trash2 className="size-3.5" />, label: "删除", onClick: onDelete },
                    ] }}
                >
                    <button type="button" className="absolute right-2 top-2 grid size-8 place-items-center rounded-[var(--r-sm)] border border-white/20 bg-black/55 text-white/85 opacity-100 shadow-sm backdrop-blur transition-[opacity,background-color] hover:bg-black/75 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/80 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100" aria-label="更多素材操作" title="更多操作">
                        <MoreHorizontal className="size-4" />
                    </button>
                </Dropdown>
                <div className="pointer-events-none absolute inset-x-2 bottom-2 flex items-center gap-1">
                    <span className="rounded-[var(--r-xs)] bg-black/60 px-1.5 py-0.5 text-[var(--fs-tiny)] font-medium text-white/90 backdrop-blur">{assetKindLabel(asset.kind)}</span>
                    <span className="rounded-[var(--r-xs)] bg-black/60 px-1.5 py-0.5 text-[var(--fs-tiny)] text-white/75 backdrop-blur">{assetCategoryLabel(asset.category)}</span>
                </div>
            </AssetLibraryCardMedia>
            <button type="button" className="block w-full px-2.5 py-2 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[var(--workspace-accent)]" onClick={onOpen}>
                <div className="flex min-w-0 items-center justify-between gap-2">
                    <h2 className="truncate text-[var(--fs-body)] font-semibold text-foreground" title={asset.title}>{asset.title}</h2>
                    <span className="shrink-0 text-[var(--fs-tiny)] tabular-nums text-foreground/38">{formatAssetTime(asset.updatedAt)}</span>
                </div>
                <div className="mt-1 truncate text-[var(--fs-label)] text-foreground/52" title={summary}>{summary}</div>
                <div className="mt-1 flex min-w-0 items-center gap-1.5 text-[var(--fs-tiny)] text-foreground/38">
                    <span className="truncate">{asset.source || "未标注来源"}</span>
                    <span aria-hidden="true">·</span>
                    <span className="truncate">{assetProjectLabel(asset)}</span>
                </div>
            </button>
        </AssetLibraryCard>
    );
}

function AssetDrawer({ asset, onClose, onCopy, onDownload }: { asset: LibraryAsset | null; onClose: () => void; onCopy: (asset: LibraryAsset) => void; onDownload: (asset: LibraryAsset) => void }) {
    return (
        <Drawer className="library-drawer" title="素材详情" open={Boolean(asset)} size="large" onClose={onClose}>
            {asset ? (
                <div className="space-y-5">
                    <AssetMediaPreview asset={asset} alt={asset.title} className="max-h-[420px] w-full rounded-lg bg-black object-contain" fallback={<div className="rounded-lg border border-stone-200 bg-stone-50 p-5 text-sm leading-6 text-stone-600 dark:border-stone-800 dark:bg-stone-900 dark:text-stone-300">{asset.kind === "text" ? asset.data.content : "暂无封面"}</div>} />
                    <div>
                        <Typography.Title level={4} className="!mb-2">
                            {asset.title}
                        </Typography.Title>
                        <Space size={[4, 4]} wrap>
                            <Tag>{assetKindLabel(asset.kind)}</Tag>
                            <Tag>{assetCategoryLabel(asset.category)}</Tag>
                            <StorageTag asset={asset} />
                            {(asset.tags || []).map((tag) => (
                                <Tag key={tag}>{tag}</Tag>
                            ))}
                        </Space>
                    </div>
                    <div className="asset-drawer-content rounded-lg border border-stone-200 p-4 dark:border-stone-800">
                        <Typography.Text type="secondary" className="block text-xs">
                            内容
                        </Typography.Text>
                        {asset.kind === "text" ? (
                            <Typography.Paragraph className="mt-2 whitespace-pre-wrap">{asset.data.content}</Typography.Paragraph>
                        ) : asset.kind === "video" ? (
                            <video src={asset.data.url} controls className="mt-2 aspect-video w-full rounded-lg bg-black" />
                        ) : asset.kind === "audio" ? (
                            <audio src={asset.data.url} controls className="mt-2 w-full" />
                        ) : asset.kind === "model" ? (
                            <Typography.Text className="mt-2 block">{asset.data.fileName} · {formatBytes(asset.data.bytes)} · {asset.data.mimeType}</Typography.Text>
                        ) : (
                            <Typography.Text className="mt-2 block">
                                {asset.data.width}x{asset.data.height} · {formatBytes(asset.data.bytes)} · {asset.data.mimeType}
                            </Typography.Text>
                        )}
                        {asset.kind === "image" || asset.kind === "video" || asset.kind === "audio" || asset.kind === "model" ? (
                            <Typography.Text type="secondary" className="mt-2 block text-xs">
                                存储位置：{resourceStorageTitle(asset.data.storageKey)}
                            </Typography.Text>
                        ) : null}
                    </div>
                    {asset.note ? (
                        <div>
                            <Typography.Text type="secondary">备注</Typography.Text>
                            <Typography.Paragraph className="mt-1">{asset.note}</Typography.Paragraph>
                        </div>
                    ) : null}
                    <Space>
                        {asset.kind === "text" ? (
                            <Button type="primary" icon={<Copy className="size-4" />} onClick={() => onCopy(asset)}>
                                复制文本
                            </Button>
                        ) : null}
                        {asset.kind === "image" || asset.kind === "video" || asset.kind === "audio" || asset.kind === "model" ? (
                            <Button type="primary" icon={<Download className="size-4" />} onClick={() => onDownload(asset)}>
                                {asset.kind === "video" ? "下载视频" : asset.kind === "audio" ? "下载音频" : asset.kind === "model" ? "下载模型" : "下载图片"}
                            </Button>
                        ) : null}
                    </Space>
                </div>
            ) : null}
        </Drawer>
    );
}

function assetSummary(asset: LibraryAsset) {
    if (asset.kind === "text") return asset.data.content;
    if (asset.kind === "audio") return `${formatAssetDuration(asset.data.durationMs)} · ${formatBytes(asset.data.bytes)} · ${asset.data.mimeType}`;
    if (asset.kind === "model") return `${asset.data.fileName} · ${formatBytes(asset.data.bytes)} · ${asset.data.mimeType}`;
    return `${asset.data.width}x${asset.data.height} · ${formatBytes(asset.data.bytes)} · ${asset.data.mimeType}`;
}

function StorageTag({ asset }: { asset: LibraryAsset }) {
    if (asset.kind !== "image" && asset.kind !== "video" && asset.kind !== "audio" && asset.kind !== "model") return null;
    const location = resourceStorageLocation(asset.data.storageKey);
    const color = location === "oss" ? "green" : location === "local" ? "gold" : "default";
    return (
        <Tag color={color} className="m-0 text-[var(--fs-label)]" title={resourceStorageTitle(asset.data.storageKey)}>
            {resourceStorageLabel(asset.data.storageKey)}
        </Tag>
    );
}

function assetSearchText(asset: LibraryAsset) {
    return [asset.title, asset.source || "", asset.note || "", assetCategoryLabel(asset.category), (asset.tags || []).join(" "), asset.kind === "text" ? asset.data.content : asset.data.mimeType].join(" ").toLowerCase();
}

function assetCategoryLabel(category?: AssetCategory) {
    return categoryOptions.find((item) => item.value === (category || "other"))?.label || "其他";
}

function assetProjectLabel(asset: LibraryAsset) {
    const projectName = asset.metadata?.projectName;
    if (typeof projectName === "string" && projectName.trim()) return projectName;
    return Array.isArray(asset.metadata?.projectIds) && asset.metadata.projectIds.length ? "已关联项目" : "未关联项目";
}

function assetKindLabel(kind: AssetKind) {
    return kind === "image" ? "图片" : kind === "video" ? "视频" : kind === "audio" ? "音频" : kind === "model" ? "3D 模型" : "文本";
}

function formatAssetDuration(durationMs?: number) {
    if (!durationMs) return "时长未知";
    return `${Math.round(durationMs / 100) / 10} 秒`;
}

function formatAssetTime(value: string) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? "-" : date.toLocaleDateString("zh-CN", { month: "2-digit", day: "2-digit" });
}
