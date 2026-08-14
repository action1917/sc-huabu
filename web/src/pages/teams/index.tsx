import { App, Button, Form, Input, Modal, Select, Tag } from "antd";
import { Copy, LogIn, Plus, RotateCcw, Search, Share2, Trash2, Users } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { PageHeader, WorkspacePage } from "@/components/layout/workspace-page";
import { WorkspaceState } from "@/components/layout/workspace-state";
import { listInspirations } from "@/services/api/inspirations";
import { listProjects } from "@/services/api/projects";
import { listResources } from "@/services/api/resources";
import { listSkills } from "@/services/api/skills";
import { createTeam, getTeam, getTeamResourceContent, joinTeam, listTeamResources, listTeams, shareTeamResource, unshareTeamResource, type Team, type TeamMember, type TeamResource } from "@/services/api/teams";
import { listRemoteAssets } from "@/services/api/user-data";
import { useUserStore } from "@/stores/use-user-store";

type TeamForm = { name: string; description: string };
type ShareForm = { sourceType: TeamResource["sourceType"]; sourceId: string; title: string };
type ShareCandidate = { value: string; label: string };
const resourceTypes = [
    { value: "work", label: "作品" },
    { value: "asset", label: "素材" },
    { value: "resource", label: "资源文件" },
    { value: "project", label: "短剧项目" },
    { value: "inspiration", label: "已发布/已解锁灵感" },
    { value: "skill", label: "已创建/已解锁技能" },
];
const resourceFilterOptions = [{ value: "all", label: "全部分类" }, ...resourceTypes];

export default function TeamsPage() {
    const { message } = App.useApp();
    const actor = useUserStore((state) => state.user);
    const [teams, setTeams] = useState<Team[]>([]);
    const [active, setActive] = useState<Team | null>(null);
    const [members, setMembers] = useState<TeamMember[]>([]);
    const [resources, setResources] = useState<TeamResource[]>([]);
    const [content, setContent] = useState<Record<string, unknown> | null>(null);
    const [loading, setLoading] = useState(true);
    const [createOpen, setCreateOpen] = useState(false);
    const [joinOpen, setJoinOpen] = useState(false);
    const [shareOpen, setShareOpen] = useState(false);
    const [saving, setSaving] = useState(false);
    const [candidateLoading, setCandidateLoading] = useState(false);
    const [shareCandidates, setShareCandidates] = useState<ShareCandidate[]>([]);
    const [search, setSearch] = useState("");
    const [resourceType, setResourceType] = useState("all");
    const [teamForm] = Form.useForm<TeamForm>();
    const [shareForm] = Form.useForm<ShareForm>();
    const shareType = Form.useWatch("sourceType", shareForm) || "work";
    const [joinCode, setJoinCode] = useState("");
    const load = async () => {
        setLoading(true);
        try {
            const { items } = await listTeams();
            const normalizedTeams = items || [];
            setTeams(normalizedTeams);
            const next = active && normalizedTeams.some((i) => i.id === active.id) ? active : normalizedTeams[0] || null;
            setActive(next);
            if (next) {
                const [detail, shared] = await Promise.all([getTeam(next.id), listTeamResources(next.id)]);
                setMembers(detail.members || []);
                setResources(shared.items || []);
            } else {
                setMembers([]);
                setResources([]);
            }
        } catch (e) {
            message.error(e instanceof Error ? e.message : "读取团队失败");
        } finally {
            setLoading(false);
        }
    };
    useEffect(() => {
        void load();
    }, []);
    useEffect(() => {
        if (!shareOpen) return;
        let activeRequest = true;
        setCandidateLoading(true);
        setShareCandidates([]);
        shareForm.setFieldValue("sourceId", undefined);
        void loadShareCandidates(shareType)
            .then((items) => activeRequest && setShareCandidates(items))
            .catch((error) => activeRequest && message.error(error instanceof Error ? error.message : "读取可共享资源失败"))
            .finally(() => activeRequest && setCandidateLoading(false));
        return () => {
            activeRequest = false;
        };
    }, [message, shareForm, shareOpen, shareType]);
    const select = async (team: Team) => {
        setActive(team);
        setLoading(true);
        try {
            const [detail, shared] = await Promise.all([getTeam(team.id), listTeamResources(team.id)]);
            setMembers(detail.members || []);
            setResources(shared.items || []);
        } catch (e) {
            message.error(e instanceof Error ? e.message : "读取团队空间失败");
        } finally {
            setLoading(false);
        }
    };
    const submitCreate = async (v: TeamForm) => {
        setSaving(true);
        try {
            await createTeam(v);
            setCreateOpen(false);
            teamForm.resetFields();
            message.success("团队已创建");
            await load();
        } catch (e) {
            message.error(e instanceof Error ? e.message : "创建团队失败");
        } finally {
            setSaving(false);
        }
    };
    const submitJoin = async () => {
        if (!joinCode.trim()) return;
        setSaving(true);
        try {
            const { team, joined } = await joinTeam(joinCode);
            setJoinOpen(false);
            setJoinCode("");
            message.success(joined ? `已加入 ${team.name}` : "你已在该团队中");
            await load();
        } catch (e) {
            message.error(e instanceof Error ? e.message : "加入团队失败");
        } finally {
            setSaving(false);
        }
    };
    const submitShare = async (v: ShareForm) => {
        if (!active) return;
        setSaving(true);
        try {
            await shareTeamResource(active.id, v);
            setShareOpen(false);
            shareForm.resetFields();
            setResources((await listTeamResources(active.id)).items || []);
            message.success("已公开到团队空间");
        } catch (e) {
            message.error(e instanceof Error ? e.message : "共享失败");
        } finally {
            setSaving(false);
        }
    };
    const removeShare = async (item: TeamResource) => {
        if (!active) return;
        try {
            await unshareTeamResource(active.id, item.sourceType, item.sourceId);
            setResources((current) => current.filter((entry) => entry.id !== item.id));
            message.success("已撤销团队共享");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "撤销共享失败");
        }
    };
    const filteredResources = useMemo(() => {
        const keyword = search.trim().toLocaleLowerCase("zh-CN");
        return resources.filter((item) => {
            if (resourceType !== "all" && item.sourceType !== resourceType) return false;
            if (!keyword) return true;
            const typeLabel = resourceTypes.find((type) => type.value === item.sourceType)?.label || item.sourceType;
            return [item.title, item.sharerName, typeLabel].some((value) => value.toLocaleLowerCase("zh-CN").includes(keyword));
        });
    }, [resourceType, resources, search]);
    const filtersActive = Boolean(search || resourceType !== "all");

    return (
        <WorkspacePage className="library-page teams-library-page" grid>
            <PageHeader icon="assets" title="团队空间" description="可加入多个团队；只有你明确公开的作品、素材、灵感和技能才会被团队成员看到。" meta={<span className="text-xs text-foreground/45">{teams.length} 个团队</span>} actions={null} />

            <div className="mt-1 flex flex-col border-b border-border/75 xl:flex-row xl:items-end xl:justify-between">
                <nav className="thin-scrollbar -mb-px flex min-w-0 overflow-x-auto" aria-label="我的团队" role="tablist">
                    {teams.length ? (
                        teams.map((team) => {
                            const selected = active?.id === team.id;
                            return (
                                <button
                                    key={team.id}
                                    type="button"
                                    role="tab"
                                    aria-selected={selected}
                                    className={`relative inline-flex h-12 max-w-48 shrink-0 items-center gap-2 border-b-2 px-3 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${selected ? "border-foreground font-medium text-foreground" : "border-transparent text-foreground/48 hover:text-foreground/76"}`}
                                    onClick={() => void select(team)}
                                >
                                    <Users className="size-4 shrink-0" />
                                    <span className="truncate">{team.name}</span>
                                </button>
                            );
                        })
                    ) : (
                        <span className="inline-flex h-12 items-center gap-2 px-3 text-sm text-foreground/42">
                            <Users className="size-4" />
                            我的团队
                        </span>
                    )}
                </nav>

                <div className="library-toolbar-controls flex min-w-0 flex-wrap items-center gap-2 py-2.5 xl:flex-nowrap xl:justify-end">
                    <Input className="w-full sm:!w-56" prefix={<Search className="size-4 text-foreground/38" />} value={search} allowClear placeholder="搜索共享内容" onChange={(event) => setSearch(event.target.value)} />
                    <Select className="w-36" value={resourceType} options={resourceFilterOptions} onChange={setResourceType} />
                    {filtersActive ? (
                        <Button
                            type="text"
                            aria-label="重置筛选"
                            title="重置筛选"
                            icon={<RotateCcw className="size-4" />}
                            onClick={() => {
                                setSearch("");
                                setResourceType("all");
                            }}
                        />
                    ) : null}
                    <Button icon={<LogIn className="size-4" />} onClick={() => setJoinOpen(true)}>
                        加入团队
                    </Button>
                    <Button icon={<Plus className="size-4" />} onClick={() => setCreateOpen(true)}>
                        创建团队
                    </Button>
                    {active ? (
                        <Button type="primary" icon={<Share2 className="size-4" />} onClick={() => setShareOpen(true)}>
                            公开资源
                        </Button>
                    ) : null}
                </div>
            </div>

            <section className="min-w-0 py-5">
                {active ? (
                    <>
                        <div className="flex flex-wrap items-center justify-between gap-3 pb-5">
                            <div className="min-w-0">
                                <div className="flex flex-wrap items-center gap-2">
                                    <h2 className="truncate text-base font-semibold">{active.name}</h2>
                                    <Tag icon={<Users className="size-3" />}>{members.length} 位成员</Tag>
                                </div>
                                <p className="mt-1 truncate text-xs text-foreground/50">{active.description || "团队共享创作空间"}</p>
                            </div>
                            <Button size="small" type="text" icon={<Copy className="size-3.5" />} onClick={() => void navigator.clipboard.writeText(active.joinCode).then(() => message.success("团队邀请码已复制"))}>
                                邀请码 {active.joinCode}
                            </Button>
                        </div>
                        {filteredResources.length ? (
                            <div className="library-grid skill-library-grid">
                                {filteredResources.map((item) => (
                                    <div key={item.id} className="relative">
                                        <button
                                            type="button"
                                            className="library-card w-full p-4 text-left"
                                            onClick={() =>
                                                void getTeamResourceContent(active.id, item.sourceType, item.sourceId)
                                                    .then(({ content }) => setContent(content))
                                                    .catch((e) => message.error(e instanceof Error ? e.message : "读取共享内容失败"))
                                            }
                                        >
                                            <div className="flex items-center justify-between gap-3">
                                                <Tag>{resourceTypes.find((type) => type.value === item.sourceType)?.label || item.sourceType}</Tag>
                                                <span className="min-w-0 truncate text-xs text-foreground/40">{item.sharerName}</span>
                                            </div>
                                            {item.previewUrl && item.sourceType === "resource" ? <img className="mt-3 aspect-video w-full rounded-md object-cover" src={item.previewUrl} alt="" /> : null}
                                            <h3 className="mt-4 truncate text-sm font-medium">{item.title}</h3>
                                            <p className="mt-1 truncate text-xs text-foreground/45">点击查看团队共享内容</p>
                                        </button>
                                        {item.sharedBy === actor?.id ? (
                                            <Button className="absolute right-2 top-2" type="text" size="small" danger aria-label="撤销团队共享" title="撤销团队共享" icon={<Trash2 className="size-3.5" />} onClick={() => void removeShare(item)} />
                                        ) : null}
                                    </div>
                                ))}
                            </div>
                        ) : !loading ? (
                            <WorkspaceState
                                compact
                                icon="assets"
                                title={filtersActive ? "没有找到匹配的共享内容" : "团队空间暂无共享内容"}
                                description={filtersActive ? "换个关键词或资源分类试试。" : "团队成员可以把自己的作品、素材、已解锁内容或技能公开到这里。"}
                                action={
                                    filtersActive ? (
                                        <Button
                                            onClick={() => {
                                                setSearch("");
                                                setResourceType("all");
                                            }}
                                        >
                                            清除筛选
                                        </Button>
                                    ) : (
                                        <Button type="primary" icon={<Share2 className="size-4" />} onClick={() => setShareOpen(true)}>
                                            公开第一个资源
                                        </Button>
                                    )
                                }
                            />
                        ) : null}
                    </>
                ) : (
                    <WorkspaceState
                        compact
                        icon="assets"
                        title="还没有加入团队"
                        description="创建团队或输入邀请码加入团队，就能协作共享资源。"
                        action={
                            <div className="flex gap-2">
                                <Button onClick={() => setJoinOpen(true)}>加入团队</Button>
                                <Button type="primary" onClick={() => setCreateOpen(true)}>
                                    创建团队
                                </Button>
                            </div>
                        }
                    />
                )}
            </section>
            <Modal open={createOpen} title="创建团队" okText="创建团队" confirmLoading={saving} onCancel={() => setCreateOpen(false)} onOk={() => teamForm.submit()}>
                <Form form={teamForm} layout="vertical" onFinish={submitCreate}>
                    <Form.Item name="name" label="团队名称" rules={[{ required: true, message: "请输入团队名称" }]}>
                        <Input maxLength={80} />
                    </Form.Item>
                    <Form.Item name="description" label="团队说明">
                        <Input.TextArea maxLength={500} rows={3} />
                    </Form.Item>
                </Form>
            </Modal>
            <Modal open={joinOpen} title="加入团队" okText="加入团队" confirmLoading={saving} onCancel={() => setJoinOpen(false)} onOk={() => void submitJoin()}>
                <Input value={joinCode} onChange={(e) => setJoinCode(e.target.value.toUpperCase())} placeholder="输入团队邀请码" maxLength={20} />
            </Modal>
            <Modal open={shareOpen} title="公开资源到团队" okText="确认公开" confirmLoading={saving} onCancel={() => setShareOpen(false)} onOk={() => shareForm.submit()}>
                <p className="mb-4 text-xs leading-5 text-foreground/55">公开后团队成员可查看和使用；收费内容仅允许作者或已付费解锁用户共享。撤销共享不会删除原资源。</p>
                <Form form={shareForm} layout="vertical" onFinish={submitShare} initialValues={{ sourceType: "work" }}>
                    <Form.Item name="sourceType" label="资源类型" rules={[{ required: true }]}>
                        <Select options={resourceTypes} />
                    </Form.Item>
                    <Form.Item name="sourceId" label="选择要公开的内容" extra="仅展示你拥有或已经解锁、可以共享到团队的内容。" rules={[{ required: true, message: "请选择要共享的内容" }]}>
                        <Select
                            showSearch
                            allowClear
                            loading={candidateLoading}
                            options={shareCandidates}
                            optionFilterProp="label"
                            placeholder={candidateLoading ? "正在读取可共享内容…" : shareCandidates.length ? "搜索并选择内容" : "暂无可共享内容"}
                            notFoundContent={candidateLoading ? "正在读取…" : "暂无可共享内容"}
                        />
                    </Form.Item>
                    <Form.Item name="title" label="团队内显示名称">
                        <Input maxLength={160} placeholder="留空则使用原名称" />
                    </Form.Item>
                </Form>
            </Modal>
            <Modal open={Boolean(content)} title={String(content?.title || "团队共享内容")} footer={<Button onClick={() => setContent(null)}>关闭</Button>} onCancel={() => setContent(null)}>
                <pre className="thin-scrollbar max-h-[60vh] overflow-auto whitespace-pre-wrap break-words rounded-md border border-border bg-muted/20 p-4 text-xs leading-6">{content ? JSON.stringify(content, null, 2) : ""}</pre>
            </Modal>
        </WorkspacePage>
    );
}

async function loadShareCandidates(sourceType: TeamResource["sourceType"]): Promise<ShareCandidate[]> {
    switch (sourceType) {
        case "work":
        case "asset": {
            const { assets } = await listRemoteAssets();
            return (assets || []).map((item) => ({ value: item.id, label: `${item.title || "未命名内容"} · ${item.kind || "资产"}` }));
        }
        case "resource": {
            const { resources } = await listResources();
            return (resources || []).filter((item) => item.status === "ready").map((item) => ({ value: item.id, label: `${fileNameFromObjectKey(item.objectKey) || item.id} · ${item.kind}` }));
        }
        case "project": {
            const { projects } = await listProjects();
            return (projects || []).map(({ project }) => ({ value: project.id, label: project.name || "未命名短剧项目" }));
        }
        case "inspiration": {
            const { items } = await listInspirations({ limit: 200 });
            return (items || []).filter((item) => item.owner || item.unlocked).map((item) => ({ value: item.id, label: `${item.title} · ${item.owner ? "我发布的" : "已解锁"}` }));
        }
        case "skill": {
            const { skills } = await listSkills({ scope: "mine", sort: "updated", page_size: 80 });
            return (skills || []).filter((item) => item.is_owner || item.unlocked).map((item) => ({ value: item.skill_id, label: `${item.skill_name} · ${item.is_owner ? "我创建的" : "已解锁"}` }));
        }
    }
    return [];
}

function fileNameFromObjectKey(value: string) {
    return value.split("/").filter(Boolean).pop() || "";
}
