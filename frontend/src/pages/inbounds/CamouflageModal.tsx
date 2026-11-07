import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Form, Input, InputNumber, Modal, Select, Space, Tag, message } from 'antd';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { DBInboundRecord } from './list/types';

interface CamouflageTemplate {
  key: string;
  name: string;
}

interface CamouflageStatus {
  enabled: boolean;
  running: boolean;
  port: number;
  template: string;
  redirectUrl?: string;
  masterInboundId?: number;
  server: string;
  templates: CamouflageTemplate[];
}

interface CamouflageModalProps {
  open: boolean;
  dbInbounds: DBInboundRecord[];
  onClose: () => void;
}

// Only these protocols can host the cover-site fallback (inboundCanHostFallbacks).
function canHostCover(ib: DBInboundRecord): boolean {
  if (ib.protocol !== 'vless' && ib.protocol !== 'trojan') return false;
  let stream: { network?: string; security?: string } = {};
  try {
    stream =
      typeof ib.streamSettings === 'string'
        ? JSON.parse(ib.streamSettings)
        : ((ib.streamSettings ?? {}) as typeof stream);
  } catch {
    return false;
  }
  const network = (stream.network || '').toLowerCase();
  return (
    (network === 'tcp' || network === 'raw') &&
    (stream.security === 'tls' || stream.security === 'reality')
  );
}

export default function CamouflageModal({ open, dbInbounds, onClose }: CamouflageModalProps) {
  const { t } = useTranslation();
  const [messageApi, contextHolder] = message.useMessage();
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<CamouflageStatus | null>(null);
  const [template, setTemplate] = useState<string>('');
  const [port, setPort] = useState<number>(2082);
  const [redirectUrl, setRedirectUrl] = useState('');
  const [masterId, setMasterId] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    const msg = await HttpUtil.get<CamouflageStatus>('/panel/api/camouflage/status', undefined, {
      silent: true,
    });
    if (msg.success && msg.obj) {
      setStatus(msg.obj);
      setPort((p) => (p === 2082 && msg.obj?.port ? msg.obj.port : p));
      setTemplate((cur) => cur || msg.obj?.template || '');
      setRedirectUrl((cur) => cur || msg.obj?.redirectUrl || '');
      setMasterId((cur) => (cur == null ? (msg.obj?.masterInboundId ?? null) : cur));
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    void (async () => {
      await load();
    })();
  }, [open, load]);

  const masterOptions = useMemo(
    () =>
      dbInbounds.filter(canHostCover).map((ib) => ({
        value: ib.id,
        label: `${ib.remark || `${ib.protocol}:${ib.port}`} · ${ib.protocol}:${ib.port}`,
      })),
    [dbInbounds],
  );

  const apply = async () => {
    setBusy(true);
    try {
      const msg = await HttpUtil.post('/panel/api/camouflage/apply', {
        template,
        port,
        redirectUrl,
        masterInboundId: masterId ?? 0,
      });
      if (msg.success) {
        await load();
        await queryClient.invalidateQueries({ queryKey: keys.inbounds.root() });
      } else if (msg.msg) {
        messageApi.error(msg.msg);
      }
    } finally {
      setBusy(false);
    }
  };

  const disable = async () => {
    setBusy(true);
    try {
      const msg = await HttpUtil.post('/panel/api/camouflage/disable', {});
      if (msg.success) {
        await load();
        await queryClient.invalidateQueries({ queryKey: keys.inbounds.root() });
      } else if (msg.msg) {
        messageApi.error(msg.msg);
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      title={t('pages.camouflage.title')}
      open={open}
      onCancel={onClose}
      footer={[
        <Button
          key="disable"
          danger
          disabled={!status?.enabled}
          loading={busy}
          onClick={() => void disable()}
        >
          {t('pages.camouflage.disable')}
        </Button>,
        <Button key="apply" type="primary" loading={busy} onClick={() => void apply()}>
          {t('pages.camouflage.apply')}
        </Button>,
      ]}
      destroyOnHidden
    >
      {contextHolder}
      <Form layout="vertical">
        <Form.Item>
          {status?.running ? (
            <Tag color="green">
              {t('pages.camouflage.statusRunning', { server: status.server })}
            </Tag>
          ) : (
            <Tag>{t('pages.camouflage.statusStopped')}</Tag>
          )}
        </Form.Item>
        <Form.Item label={t('pages.camouflage.template')} htmlFor="cm-template">
          <Select
            id="cm-template"
            allowClear
            value={template || undefined}
            placeholder={t('pages.camouflage.none')}
            onChange={(v) => setTemplate(v ?? '')}
            options={(status?.templates ?? []).map((tpl) => ({ value: tpl.key, label: tpl.name }))}
          />
        </Form.Item>
        <Form.Item label={t('pages.camouflage.port')} htmlFor="cm-port">
          <InputNumber
            id="cm-port"
            min={1}
            max={65534}
            style={{ width: '100%' }}
            value={port}
            onChange={(v) => setPort(v ?? 2082)}
          />
        </Form.Item>
        <Form.Item label={t('pages.camouflage.redirectUrl')} htmlFor="cm-redirect">
          <Input
            id="cm-redirect"
            value={redirectUrl}
            placeholder="https://example.com"
            onChange={(e) => setRedirectUrl(e.target.value)}
          />
        </Form.Item>
        <Form.Item
          label={t('pages.camouflage.masterInbound')}
          htmlFor="cm-master"
          extra={t('pages.camouflage.masterHint')}
        >
          <Select
            id="cm-master"
            allowClear
            value={masterId ?? undefined}
            placeholder={t('pages.camouflage.none')}
            onChange={(v) => setMasterId(v ?? null)}
            options={masterOptions}
          />
        </Form.Item>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Alert type="info" showIcon message={t('pages.camouflage.hint')} />
        </Space>
      </Form>
    </Modal>
  );
}
