import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import { Alert, Form, Input, InputNumber, Modal, Select, message } from 'antd';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

interface QuickTemplate {
  key: string;
  name: string;
  protocol: string;
  needsCert: boolean;
  summary: string;
}

interface QuickAddModalProps {
  open: boolean;
  onClose: () => void;
}

export default function QuickAddModal({ open, onClose }: QuickAddModalProps) {
  const { t } = useTranslation();
  const [messageApi, contextHolder] = message.useMessage();
  const queryClient = useQueryClient();
  const [templates, setTemplates] = useState<QuickTemplate[]>([]);
  const [template, setTemplate] = useState<string>('');
  const [port, setPort] = useState<number | null>(null);
  const [remark, setRemark] = useState('');
  const [certFile, setCertFile] = useState('');
  const [keyFile, setKeyFile] = useState('');
  const [serverName, setServerName] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open || templates.length > 0) return;
    void (async () => {
      const msg = await HttpUtil.get<QuickTemplate[]>(
        '/panel/api/inbounds/quickTemplates',
        undefined,
        {
          silent: true,
        },
      );
      if (msg.success && Array.isArray(msg.obj)) {
        setTemplates(msg.obj);
        setTemplate((cur) => cur || msg.obj?.[0]?.key || '');
      }
    })();
  }, [open, templates.length]);

  const selected = useMemo(
    () => templates.find((tpl) => tpl.key === template),
    [templates, template],
  );

  const handleOk = async () => {
    if (!template) return;
    setSubmitting(true);
    try {
      const msg = await HttpUtil.post('/panel/api/inbounds/quickAdd', {
        template,
        port: port ?? 0,
        remark,
        certFile,
        keyFile,
        serverName,
      });
      if (msg.success) {
        await queryClient.invalidateQueries({ queryKey: keys.inbounds.root() });
        await queryClient.invalidateQueries({ queryKey: keys.clients.root() });
        onClose();
      } else if (msg.msg) {
        messageApi.error(msg.msg);
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={t('pages.inbounds.quickAddTitle')}
      open={open}
      onCancel={onClose}
      onOk={handleOk}
      okButtonProps={{ loading: submitting, disabled: !template }}
      destroyOnHidden
    >
      {contextHolder}
      <Form layout="vertical">
        <Form.Item label={t('pages.inbounds.quickTemplate')} htmlFor="qa-template">
          <Select
            id="qa-template"
            value={template || undefined}
            onChange={setTemplate}
            options={templates.map((tpl) => ({
              value: tpl.key,
              label: tpl.needsCert ? `${tpl.name} ✱` : tpl.name,
            }))}
          />
        </Form.Item>
        {selected?.summary ? (
          <Alert type="info" showIcon style={{ marginBottom: 16 }} message={selected.summary} />
        ) : null}
        {selected?.needsCert ? (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
            message={t('pages.inbounds.quickCertHint')}
          />
        ) : null}
        <Form.Item label={t('pages.inbounds.quickPortOptional')} htmlFor="qa-port">
          <InputNumber
            id="qa-port"
            min={1}
            max={65534}
            style={{ width: '100%' }}
            value={port}
            onChange={(v) => setPort(v)}
          />
        </Form.Item>
        <Form.Item label={t('pages.inbounds.quickRemarkOptional')} htmlFor="qa-remark">
          <Input id="qa-remark" value={remark} onChange={(e) => setRemark(e.target.value)} />
        </Form.Item>
        {selected?.needsCert ? (
          <>
            <Form.Item label="certFile" htmlFor="qa-cert">
              <Input
                id="qa-cert"
                value={certFile}
                placeholder="/etc/ssl/certs/site.crt"
                onChange={(e) => setCertFile(e.target.value)}
              />
            </Form.Item>
            <Form.Item label="keyFile" htmlFor="qa-key">
              <Input
                id="qa-key"
                value={keyFile}
                placeholder="/etc/ssl/private/site.key"
                onChange={(e) => setKeyFile(e.target.value)}
              />
            </Form.Item>
            <Form.Item label="serverName (SNI)" htmlFor="qa-sni">
              <Input
                id="qa-sni"
                value={serverName}
                placeholder="site.example.com"
                onChange={(e) => setServerName(e.target.value)}
              />
            </Form.Item>
          </>
        ) : null}
      </Form>
    </Modal>
  );
}
