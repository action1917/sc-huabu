import { Alert, App, Button, Collapse, Form, Input, InputNumber, Space, Switch, Tag } from "antd";
import { CreditCard, Globe2, Plus, ShieldCheck, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";

import { getAdminPaymentSetting, updateAdminPaymentSetting, type PaymentSetting } from "@/services/api/wallet";
import { AdminPageFrame } from "../components/admin-shell";
import { configuredSecretText, SettingsSectionCard } from "../components/admin-ui";

type PackageFormValue = { id: string; name: string; creditAmount: number; cnyAmount: number; usdAmount: number };
type PaymentFormValues = Omit<PaymentSetting, "packages"> & { packages: PackageFormValue[] };

export default function PaymentSettingsPage() {
    const { message } = App.useApp();
    const [form] = Form.useForm<PaymentFormValues>();
    const [setting, setSetting] = useState<PaymentSetting | null>(null);
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const enabled = Form.useWatch("enabled", form);
    const alipayEnabled = Form.useWatch(["alipay", "enabled"], form);
    const wechatEnabled = Form.useWatch(["wechat", "enabled"], form);
    const stripeEnabled = Form.useWatch(["stripe", "enabled"], form);
    const paypalEnabled = Form.useWatch(["paypal", "enabled"], form);
    const genericEnabled = Form.useWatch(["generic", "enabled"], form);

    useEffect(() => {
        void getAdminPaymentSetting()
            .then(({ setting: value }) => {
                setSetting(value);
                form.setFieldsValue(toFormValues(value));
            })
            .catch((error) => message.error(error instanceof Error ? error.message : "读取支付配置失败"))
            .finally(() => setLoading(false));
    }, [form, message]);

    const save = async () => {
        const values = await form.validateFields();
        setSaving(true);
        try {
            const result = await updateAdminPaymentSetting({
                ...values,
                baseUrl: values.baseUrl.trim().replace(/\/$/, ""),
                packages: values.packages.map((item) => ({
                    id: item.id.trim(),
                    name: item.name.trim(),
                    creditMicrocredits: Math.round(item.creditAmount * 1_000_000),
                    cnyAmountMinor: Math.round(item.cnyAmount * 100),
                    usdAmountMinor: Math.round(item.usdAmount * 100),
                })),
            });
            setSetting(result.setting);
            form.setFieldsValue(toFormValues(result.setting));
            message.success("支付配置已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存支付配置失败");
        } finally {
            setSaving(false);
        }
    };

    return (
        <AdminPageFrame title="支付与充值" description="支付宝、微信、Stripe、PayPal 与通用支付网关">
            <div className="space-y-4 pt-4">
                <Alert showIcon type="warning" message="启用前必须配置 HTTPS 公网域名" description="支付平台会从公网回调本站；系统只在服务端解密和使用密钥，浏览器不会读回明文。建议先用各平台沙箱完成小额支付、重复回调和退款流程验证。" />
                <SettingsSectionCard
                    icon={<CreditCard className="size-4" />}
                    title="在线充值"
                    description="配置充值套餐、公开回调地址和收款渠道。"
                    status={
                        <Space size={6}>
                            <Tag color={enabled ? "success" : "default"}>{enabled ? "已启用" : "已停用"}</Tag>
                            <Tag color="blue">
                                <ShieldCheck className="mr-1 inline size-3" />
                                密钥加密保存
                            </Tag>
                        </Space>
                    }
                    footer={
                        <>
                            <span className="text-xs text-foreground/45">{setting?.updatedAt ? `上次更新：${new Date(setting.updatedAt).toLocaleString("zh-CN", { hour12: false })}` : "尚未保存支付配置"}</span>
                            <Button type="primary" loading={saving} onClick={() => void save()}>
                                保存支付配置
                            </Button>
                        </>
                    }
                >
                    <Form form={form} layout="vertical" requiredMark={false} disabled={loading} className="p-5">
                        <div className="grid gap-x-5 md:grid-cols-2">
                            <Form.Item name="enabled" label="在线支付总开关" valuePropName="checked">
                                <Switch checkedChildren="开启" unCheckedChildren="关闭" />
                            </Form.Item>
                            <Form.Item
                                name="baseUrl"
                                label="HTTPS 公网基础地址"
                                tooltip="不含 /api，例如 https://canvas.example.com"
                                rules={
                                    enabled
                                        ? [
                                              { required: true, message: "请填写 HTTPS 公网基础地址" },
                                              { type: "url", message: "请输入完整 URL" },
                                          ]
                                        : []
                                }
                            >
                                <Input prefix={<Globe2 className="size-4 text-foreground/35" />} placeholder="https://canvas.example.com" />
                            </Form.Item>
                        </div>

                        <div className="mb-2 text-sm font-semibold">充值套餐</div>
                        <Form.List name="packages">
                            {(fields, { add, remove }) => (
                                <div className="space-y-3">
                                    {fields.map((field) => (
                                        <div key={field.key} className="grid items-end gap-3 rounded-lg border border-border/70 bg-muted/20 p-4 md:grid-cols-[1fr_1.3fr_1fr_1fr_1fr_auto]">
                                            <Form.Item {...field} name={[field.name, "id"]} label="套餐 ID" rules={[{ required: true }]} className="mb-0">
                                                <Input placeholder="starter" />
                                            </Form.Item>
                                            <Form.Item {...field} name={[field.name, "name"]} label="套餐名称" rules={[{ required: true }]} className="mb-0">
                                                <Input placeholder="入门包" />
                                            </Form.Item>
                                            <Form.Item {...field} name={[field.name, "creditAmount"]} label="到账积分" rules={[{ required: true }]} className="mb-0">
                                                <InputNumber min={0.000001} precision={6} className="w-full" />
                                            </Form.Item>
                                            <Form.Item {...field} name={[field.name, "cnyAmount"]} label="人民币 ¥" rules={[{ required: true }]} className="mb-0">
                                                <InputNumber min={0} precision={2} className="w-full" />
                                            </Form.Item>
                                            <Form.Item {...field} name={[field.name, "usdAmount"]} label="美元 $" rules={[{ required: true }]} className="mb-0">
                                                <InputNumber min={0} precision={2} className="w-full" />
                                            </Form.Item>
                                            <Button danger type="text" icon={<Trash2 className="size-4" />} aria-label="删除套餐" onClick={() => remove(field.name)} />
                                        </div>
                                    ))}
                                    <Button block type="dashed" icon={<Plus className="size-4" />} onClick={() => add({ id: "", name: "", creditAmount: 100, cnyAmount: 10, usdAmount: 2 })}>
                                        添加充值套餐
                                    </Button>
                                </div>
                            )}
                        </Form.List>

                        <div className="mb-2 mt-6 text-sm font-semibold">支付渠道</div>
                        <Collapse
                            items={[
                                {
                                    key: "alipay",
                                    label: <ChannelLabel name="支付宝" enabled={alipayEnabled} />,
                                    children: (
                                        <div className="grid gap-x-5 md:grid-cols-2">
                                            <Toggle name={["alipay", "enabled"]} label="启用支付宝" />
                                            <Toggle name={["alipay", "sandbox"]} label="沙箱环境" />
                                            <Field name={["alipay", "appId"]} label="App ID" />
                                            <Field name={["alipay", "sellerId"]} label="支付宝账号 PID（可选）" />
                                            <Secret name={["alipay", "merchantPrivateKey"]} label="应用 RSA 私钥" configured={setting?.alipay.hasMerchantPrivateKey} />
                                            <TextAreaField name={["alipay", "alipayPublicKey"]} label="支付宝 RSA 公钥" />
                                        </div>
                                    ),
                                },
                                {
                                    key: "wechat",
                                    label: <ChannelLabel name="微信支付 Native" enabled={wechatEnabled} />,
                                    children: (
                                        <div className="grid gap-x-5 md:grid-cols-2">
                                            <Toggle name={["wechat", "enabled"]} label="启用微信支付" />
                                            <Field name={["wechat", "appId"]} label="App ID" />
                                            <Field name={["wechat", "merchantId"]} label="商户号 mchid" />
                                            <Field name={["wechat", "merchantSerialNo"]} label="商户证书序列号" />
                                            <Secret name={["wechat", "apiV3Key"]} label="APIv3 密钥（32 位）" configured={setting?.wechat.hasApiV3Key} />
                                            <Secret name={["wechat", "merchantPrivateKey"]} label="商户 RSA 私钥" configured={setting?.wechat.hasMerchantPrivateKey} />
                                            <Field name={["wechat", "platformPublicKeyId"]} label="平台公钥 ID / 证书序列号" />
                                            <TextAreaField name={["wechat", "platformPublicKey"]} label="微信支付平台公钥" />
                                        </div>
                                    ),
                                },
                                {
                                    key: "stripe",
                                    label: <ChannelLabel name="Stripe Checkout" enabled={stripeEnabled} />,
                                    children: (
                                        <div className="grid gap-x-5 md:grid-cols-2">
                                            <Toggle name={["stripe", "enabled"]} label="启用 Stripe" />
                                            <div />
                                            <Secret name={["stripe", "secretKey"]} label="Secret Key" configured={setting?.stripe.hasSecretKey} />
                                            <Secret name={["stripe", "webhookSecret"]} label="Webhook Signing Secret" configured={setting?.stripe.hasWebhookSecret} />
                                        </div>
                                    ),
                                },
                                {
                                    key: "paypal",
                                    label: <ChannelLabel name="PayPal" enabled={paypalEnabled} />,
                                    children: (
                                        <div className="grid gap-x-5 md:grid-cols-2">
                                            <Toggle name={["paypal", "enabled"]} label="启用 PayPal" />
                                            <Toggle name={["paypal", "sandbox"]} label="沙箱环境" />
                                            <Field name={["paypal", "clientId"]} label="Client ID" />
                                            <Secret name={["paypal", "clientSecret"]} label="Client Secret" configured={setting?.paypal.hasClientSecret} />
                                            <Field name={["paypal", "webhookId"]} label="Webhook ID" />
                                        </div>
                                    ),
                                },
                                {
                                    key: "generic",
                                    label: <ChannelLabel name="通用国际支付网关" enabled={genericEnabled} />,
                                    children: (
                                        <div className="grid gap-x-5 md:grid-cols-2">
                                            <Toggle name={["generic", "enabled"]} label="启用通用网关" />
                                            <Field name={["generic", "name"]} label="前台显示名称" />
                                            <Field name={["generic", "currency"]} label="币种（CNY 或 USD）" />
                                            <Field name={["generic", "createUrl"]} label="HTTPS 下单接口" />
                                            <Field name={["generic", "apiKeyHeader"]} label="API Key Header" />
                                            <Secret name={["generic", "apiKey"]} label="API Key" configured={setting?.generic.hasApiKey} />
                                            <Secret name={["generic", "webhookSecret"]} label="Webhook HMAC Secret" configured={setting?.generic.hasWebhookSecret} />
                                        </div>
                                    ),
                                },
                            ]}
                        />
                    </Form>
                </SettingsSectionCard>
            </div>
        </AdminPageFrame>
    );
}

function toFormValues(setting: PaymentSetting): PaymentFormValues {
    return { ...setting, packages: setting.packages.map((item) => ({ id: item.id, name: item.name, creditAmount: item.creditMicrocredits / 1_000_000, cnyAmount: item.cnyAmountMinor / 100, usdAmount: item.usdAmountMinor / 100 })) };
}
function ChannelLabel({ name, enabled }: { name: string; enabled?: boolean }) {
    return (
        <Space>
            <span>{name}</span>
            <Tag color={enabled ? "success" : "default"}>{enabled ? "已启用" : "未启用"}</Tag>
        </Space>
    );
}
function Toggle({ name, label }: { name: (string | number)[]; label: string }) {
    return (
        <Form.Item name={name} label={label} valuePropName="checked">
            <Switch />
        </Form.Item>
    );
}
function Field({ name, label }: { name: (string | number)[]; label: string }) {
    return (
        <Form.Item name={name} label={label}>
            <Input autoComplete="off" />
        </Form.Item>
    );
}
function TextAreaField({ name, label }: { name: (string | number)[]; label: string }) {
    return (
        <Form.Item name={name} label={label} className="md:col-span-2">
            <Input.TextArea autoSize={{ minRows: 3, maxRows: 8 }} autoComplete="off" />
        </Form.Item>
    );
}
function Secret({ name, label, configured }: { name: (string | number)[]; label: string; configured?: boolean }) {
    return (
        <Form.Item name={name} label={`${label}${configured ? `（${configuredSecretText}）` : ""}`}>
            <Input.Password autoComplete="new-password" placeholder={configured ? "留空保留原密钥" : "请输入密钥"} />
        </Form.Item>
    );
}
