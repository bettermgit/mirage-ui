import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  Input,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import {
  ApiOutlined,
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  NodeIndexOutlined,
  PlusOutlined,
  ReloadOutlined,
  LinkOutlined,
} from '@ant-design/icons';

import { SettingListItem } from '@/components/ui';
import { selectTagRender } from '@/components/form/SelectTag';
import { getMessage } from '@/utils/messageBus';
import { ClipboardManager } from '@/utils';
import { publicationURL } from '@/lib/publicationUrl';
import JsonEditor from '@/components/form/JsonEditor';
import {
  useSubConvMutations,
  useSubConvStateQuery,
  useWorkbenchClientsQuery,
} from '@/api/queries/useSubConvWorkbench';
import type { AllSetting } from '@/models/setting';
import type { SubConvPublication, SubConvSource } from '@/generated/zod';
import SourceFormModal from './SourceFormModal';
import PublicationFormModal from './PublicationFormModal';
import RuleSourceCard from './RuleSourceCard';
import type { SourceFormValues, PublicationFormValues } from './workbenchSchemas';

interface MihomoEngineTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

type EngineDoc = Record<string, unknown>;

function parseDoc(raw: string): EngineDoc | null {
  if (!raw || !raw.trim()) return {};
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function formatTime(unixSeconds: number): string {
  if (!unixSeconds) return '-';
  return new Date(unixSeconds * 1000).toLocaleString();
}

export default function MihomoEngineTab({ allSetting, updateSetting }: MihomoEngineTabProps) {
  const { t } = useTranslation();
  const messageApi = getMessage();
  const { sources, publications, loading, fetchError, refetch } = useSubConvStateQuery();
  const { data: clients } = useWorkbenchClientsQuery();
  const mutations = useSubConvMutations();

  const pubURL = (token: string) =>
    publicationURL({
      token,
      subClashURI: allSetting.subClashURI,
      subURI: allSetting.subURI,
      subPort: allSetting.subPort,
    });

  const [sourceModal, setSourceModal] = useState(false);
  const [editingSource, setEditingSource] = useState<SubConvSource | null>(null);
  const [pubModal, setPubModal] = useState(false);
  const [editingPub, setEditingPub] = useState<SubConvPublication | null>(null);
  const [previewSources, setPreviewSources] = useState<string[]>([]);
  const [previewClients, setPreviewClients] = useState<string[]>([]);
  const [previewYaml, setPreviewYaml] = useState<string | null>(null);
  const [previewCount, setPreviewCount] = useState(0);
  const [previewing, setPreviewing] = useState(false);

  const doc = useMemo(() => parseDoc(allSetting.subConvConf), [allSetting.subConvConf]);
  const [draftJson, setDraftJson] = useState<string | null>(null);

  const commit = (next: EngineDoc) => {
    updateSetting({ subConvConf: Object.keys(next).length > 0 ? JSON.stringify(next) : '' });
  };

  const setField = (key: string, value: unknown) => {
    const next: EngineDoc = { ...(doc ?? {}) };
    if (value === undefined) delete next[key];
    else next[key] = value;
    commit(next);
  };

  const applyPatch = (patch: Record<string, unknown>) => {
    const next: EngineDoc = { ...(doc ?? {}) };
    for (const [key, value] of Object.entries(patch)) {
      if (value === undefined) delete next[key];
      else next[key] = value;
    }
    commit(next);
  };

  const groupOptions =
    typeof doc?.group_options === 'object' && doc.group_options !== null
      ? (doc.group_options as EngineDoc)
      : {};
  const regionGroups = Boolean(groupOptions.enable_region_groups);

  const setRegionGroups = (on: boolean) => {
    setField('group_options', { ...groupOptions, enable_region_groups: on });
  };

  const jsonText = draftJson ?? (allSetting.subConvConf || JSON.stringify(doc ?? {}, null, 2));

  const applyJson = () => {
    if (draftJson === null) return;
    const trimmed = draftJson.trim();
    if (!trimmed) {
      updateSetting({ subConvConf: '' });
      setDraftJson(null);
      return;
    }
    try {
      const parsed = JSON.parse(trimmed);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error();
      updateSetting({ subConvConf: JSON.stringify(parsed) });
      setDraftJson(null);
    } catch {
      messageApi.error(t('pages.subscription.invalidJson'));
    }
  };

  const toast = (msg: { success?: boolean; msg?: string } | undefined, ok: string) => {
    if (msg?.success) messageApi.success(msg.msg && msg.msg !== 'ok' ? msg.msg : t(ok));
    else if (msg && msg.success === false) messageApi.error(msg.msg || t('somethingWentWrong'));
  };

  async function onSourceConfirm(values: SourceFormValues) {
    const msg = await mutations.saveSource({ ...values, id: editingSource?.id ?? '' });
    toast(msg, 'pages.inbounds.toasts.updateSuccess');
    if (msg?.success) setSourceModal(false);
  }

  async function onPubConfirm(values: PublicationFormValues) {
    const { rotateToken, ...rest } = values;
    const msg = await mutations.savePublication(
      { ...rest, id: editingPub?.id ?? '', token: editingPub?.token ?? '' },
      rotateToken,
    );
    toast(msg, 'pages.inbounds.toasts.updateSuccess');
    if (msg?.success) setPubModal(false);
  }

  async function toggleSource(source: SubConvSource) {
    const msg = await mutations.saveSource({ ...source, enable: !source.enable });
    toast(msg, 'pages.inbounds.toasts.updateSuccess');
  }

  async function refreshSource(source: SubConvSource) {
    const msg = await mutations.refreshSource(source.id);
    toast(msg, 'pages.inbounds.toasts.updateSuccess');
  }

  async function copyLink(publication: SubConvPublication) {
    const url = pubURL(publication.token);
    const ok = await ClipboardManager.copyText(url);
    if (ok) messageApi.success(t('pages.subscription.linkCopied'));
    else messageApi.error(t('somethingWentWrong'));
  }

  async function runPreview() {
    setPreviewing(true);
    try {
      const msg = await mutations.preview(previewSources, previewClients);
      if (msg?.success && msg.obj) {
        const obj = msg.obj as { yaml?: string; nodeCount?: number };
        setPreviewYaml(obj.yaml ?? '');
        setPreviewCount(obj.nodeCount ?? 0);
      } else {
        setPreviewYaml(null);
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } catch (err) {
      setPreviewYaml(null);
      messageApi.error(String((err as Error)?.message ?? err));
    } finally {
      setPreviewing(false);
    }
  }

  const sourceColumns = [
    {
      title: t('pages.subscription.sourceName'),
      dataIndex: 'name',
      key: 'name',
      width: 160,
    },
    {
      title: t('pages.subscription.sourceUrl'),
      dataIndex: 'url',
      key: 'url',
      ellipsis: true,
    },
    {
      title: t('pages.subscription.fetchStatus'),
      key: 'status',
      width: 220,
      render: (_: unknown, r: SubConvSource) =>
        r.lastError ? (
          <Tooltip title={r.lastError}>
            <Tag color="red">{t('pages.subscription.fetchFailed')}</Tag>
          </Tooltip>
        ) : r.lastFetchAt ? (
          <span>
            <Tag color="green">{t('pages.subscription.fetchCount', { count: r.lastCount })}</Tag>
            {formatTime(r.lastFetchAt)}
          </span>
        ) : (
          <Tag>{t('pages.subscription.fetchNever')}</Tag>
        ),
    },
    {
      title: t('enabled'),
      key: 'enable',
      width: 72,
      align: 'center' as const,
      render: (_: unknown, r: SubConvSource) => (
        <Switch
          size="small"
          aria-label={t('enabled')}
          checked={r.enable}
          onChange={() => toggleSource(r)}
        />
      ),
    },
    {
      title: '',
      key: 'actions',
      width: 128,
      render: (_: unknown, r: SubConvSource) => (
        <Space>
          <Tooltip title={t('pages.subscription.refreshNow')}>
            <Button
              aria-label={t('pages.subscription.refreshNow')}
              size="small"
              icon={<ReloadOutlined aria-hidden />}
              onClick={() => refreshSource(r)}
            />
          </Tooltip>
          <Button
            aria-label={t('edit')}
            size="small"
            icon={<EditOutlined aria-hidden />}
            onClick={() => {
              setEditingSource(r);
              setSourceModal(true);
            }}
          />
          <Popconfirm
            title={t('pages.subscription.deleteSourceConfirm')}
            okText={t('delete')}
            cancelText={t('cancel')}
            onConfirm={async () => {
              const msg = await mutations.deleteSource(r.id);
              toast(msg, 'pages.inbounds.toasts.updateSuccess');
            }}
          >
            <Button
              aria-label={t('delete')}
              size="small"
              danger
              icon={<DeleteOutlined aria-hidden />}
            />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const pubColumns = [
    {
      title: t('pages.subscription.sourceName'),
      dataIndex: 'name',
      key: 'name',
      width: 160,
    },
    {
      title: t('pages.subscription.pubLink'),
      key: 'link',
      ellipsis: true,
      render: (_: unknown, r: SubConvPublication) => (
        <Typography.Link copyable={{ text: pubURL(r.token) }} ellipsis>
          {pubURL(r.token)}
        </Typography.Link>
      ),
    },
    {
      title: t('pages.subscription.pubComposition'),
      key: 'composition',
      width: 160,
      render: (_: unknown, r: SubConvPublication) =>
        t('pages.subscription.pubCompositionValue', {
          sources: r.sourceIds.length,
          clients: r.subIds.length,
        }),
    },
    {
      title: t('enabled'),
      key: 'enable',
      width: 72,
      align: 'center' as const,
      render: (_: unknown, r: SubConvPublication) => (
        <Switch
          size="small"
          aria-label={t('enabled')}
          checked={r.enable}
          onChange={async () => {
            const msg = await mutations.savePublication({ ...r, enable: !r.enable }, false);
            toast(msg, 'pages.inbounds.toasts.updateSuccess');
          }}
        />
      ),
    },
    {
      title: '',
      key: 'actions',
      width: 128,
      render: (_: unknown, r: SubConvPublication) => (
        <Space>
          <Tooltip title={t('pages.subscription.copyLink')}>
            <Button
              aria-label={t('pages.subscription.copyLink')}
              size="small"
              icon={<CopyOutlined aria-hidden />}
              onClick={() => copyLink(r)}
            />
          </Tooltip>
          <Button
            aria-label={t('edit')}
            size="small"
            icon={<EditOutlined aria-hidden />}
            onClick={() => {
              setEditingPub(r);
              setPubModal(true);
            }}
          />
          <Popconfirm
            title={t('pages.subscription.deletePubConfirm')}
            okText={t('delete')}
            cancelText={t('cancel')}
            onConfirm={async () => {
              const msg = await mutations.deletePublication(r.id);
              toast(msg, 'pages.inbounds.toasts.updateSuccess');
            }}
          >
            <Button
              aria-label={t('delete')}
              size="small"
              danger
              icon={<DeleteOutlined aria-hidden />}
            />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const sourceOptions = sources.map((s) => ({ label: s.name, value: s.id }));
  const clientOptions = (clients ?? []).map((c) => ({
    label: `${c.email} (${c.subId})`,
    value: c.subId,
  }));

  return (
    <div>
      {doc === null && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 10 }}
          title={t('pages.subscription.storedInvalid')}
          description={t('pages.subscription.storedInvalidDesc')}
        />
      )}
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 10 }}
        title={t('pages.subscription.workbenchIntro')}
      />
      {fetchError && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 10 }}
          title={fetchError}
          action={
            <Button size="small" onClick={() => refetch()}>
              {t('refresh')}
            </Button>
          }
        />
      )}

      <Card
        size="small"
        style={{ marginBottom: 10 }}
        title={
          <span>
            <ApiOutlined aria-hidden /> {t('pages.subscription.sourcesSection')}
          </span>
        }
        extra={
          <Button
            type="primary"
            size="small"
            icon={<PlusOutlined aria-hidden />}
            onClick={() => {
              setEditingSource(null);
              setSourceModal(true);
            }}
          >
            {t('pages.subscription.addSource')}
          </Button>
        }
      >
        <Table
          size="small"
          rowKey={(r) => r.id}
          dataSource={sources}
          columns={sourceColumns}
          pagination={false}
          loading={loading && sources.length === 0}
          scroll={{ x: true }}
          locale={{ emptyText: t('pages.subscription.emptySources') }}
        />
      </Card>

      <Card
        size="small"
        style={{ marginBottom: 10 }}
        title={
          <span>
            <LinkOutlined aria-hidden /> {t('pages.subscription.publicationsSection')}
          </span>
        }
        extra={
          <Button
            type="primary"
            size="small"
            icon={<PlusOutlined aria-hidden />}
            onClick={() => {
              setEditingPub(null);
              setPubModal(true);
            }}
          >
            {t('pages.subscription.addPublication')}
          </Button>
        }
      >
        <Table
          size="small"
          rowKey={(r) => r.id}
          dataSource={publications}
          columns={pubColumns}
          pagination={false}
          loading={loading && publications.length === 0}
          scroll={{ x: true }}
          locale={{ emptyText: t('pages.subscription.emptyPublications') }}
        />
      </Card>

      <Card
        size="small"
        style={{ marginBottom: 10 }}
        title={
          <span>
            <NodeIndexOutlined aria-hidden /> {t('pages.subscription.previewSection')}
          </span>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }} size={10}>
          <Space style={{ width: '100%' }} wrap>
            <Select
              mode="multiple"
              allowClear
              showSearch
              optionFilterProp="label"
              aria-label={t('pages.subscription.pubSources')}
              tagRender={selectTagRender}
              style={{ minWidth: 240, maxWidth: 420 }}
              placeholder={t('pages.subscription.pubSourcesPlaceholder')}
              value={previewSources}
              options={sourceOptions}
              onChange={setPreviewSources}
            />
            <Select
              mode="multiple"
              allowClear
              showSearch
              optionFilterProp="label"
              aria-label={t('pages.subscription.pubClients')}
              tagRender={selectTagRender}
              style={{ minWidth: 240, maxWidth: 420 }}
              placeholder={t('pages.subscription.pubClientsPlaceholder')}
              value={previewClients}
              options={clientOptions}
              onChange={setPreviewClients}
            />
            <Button
              type="primary"
              icon={<ReloadOutlined aria-hidden />}
              loading={previewing}
              disabled={previewSources.length + previewClients.length === 0}
              onClick={runPreview}
            >
              {t('pages.subscription.previewBtn')}
            </Button>
          </Space>
          {previewYaml !== null && (
            <>
              <div style={{ opacity: 0.65 }}>
                {t('pages.subscription.nodeCount', { count: previewCount })}
              </div>
              <Input.TextArea
                readOnly
                aria-label={t('pages.subscription.previewSection')}
                value={previewYaml}
                autoSize={{ maxRows: 24 }}
              />
            </>
          )}
        </Space>
      </Card>

      <Card
        size="small"
        style={{ marginBottom: 10 }}
        title={
          <span>
            <NodeIndexOutlined aria-hidden /> {t('pages.subscription.quickSection')}
          </span>
        }
      >
        <SettingListItem
          paddings="small"
          title={t('pages.subscription.emoji')}
          description={t('pages.subscription.emojiDesc')}
        >
          <Switch
            checked={typeof doc?.emoji === 'boolean' ? (doc.emoji as boolean) : true}
            onChange={(v) => setField('emoji', v)}
          />
        </SettingListItem>
        <SettingListItem
          paddings="small"
          title={t('pages.subscription.regionGroups')}
          description={t('pages.subscription.regionGroupsDesc')}
        >
          <Switch checked={regionGroups} onChange={setRegionGroups} />
        </SettingListItem>
      </Card>

      <RuleSourceCard doc={doc ?? {}} onPatch={applyPatch} />

      <Card
        size="small"
        title={
          <span>
            <NodeIndexOutlined aria-hidden /> {t('pages.subscription.advanced')}
          </span>
        }
        extra={
          <Button
            size="small"
            onClick={() => {
              updateSetting({ subConvConf: '' });
              setDraftJson(null);
            }}
          >
            {t('pages.subscription.reset')}
          </Button>
        }
      >
        <div className="mb-10" style={{ opacity: 0.65 }}>
          {t('pages.subscription.advancedDesc')}
        </div>
        <JsonEditor value={jsonText} onChange={setDraftJson} minHeight="260px" />
        {draftJson !== null && (
          <div style={{ marginTop: 10 }}>
            <Button type="primary" size="small" onClick={applyJson}>
              {t('pages.subscription.applyJson')}
            </Button>
            <Button size="small" style={{ marginLeft: 8 }} onClick={() => setDraftJson(null)}>
              {t('cancel')}
            </Button>
          </div>
        )}
      </Card>

      <SourceFormModal
        open={sourceModal}
        source={editingSource}
        onClose={() => setSourceModal(false)}
        onConfirm={onSourceConfirm}
      />
      <PublicationFormModal
        open={pubModal}
        publication={editingPub}
        sources={sources}
        onClose={() => setPubModal(false)}
        onConfirm={onPubConfirm}
      />
    </div>
  );
}
