import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  Input,
  Popconfirm,
  Radio,
  Select,
  Space,
  Switch,
  Tabs,
  Typography,
} from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';

import { getMessage } from '@/utils/messageBus';
import CustomRuleModal, { type CustomRuleDraft } from './CustomRuleModal';
import {
  RULE_CATALOG,
  RULE_CATALOG_KEYS,
  TEMPLATE_PRESETS,
  buildQuickAddRules,
  enabledRuleSet,
  normalizeDirectRuleLines,
  parseQuickAddLines,
  type CustomRuleEntry,
} from './ruleCatalog';

interface RuleSourceCardProps {
  doc: Record<string, unknown>;
  onPatch: (patch: Record<string, unknown>) => void;
}

const VISIBLE_CHIPS = 12;

function readStrings(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((x): x is string => typeof x === 'string') : [];
}

function readCustomRules(value: unknown): CustomRuleEntry[] {
  if (!Array.isArray(value)) return [];
  return value.filter((x): x is CustomRuleEntry => Boolean(x) && typeof x === 'object');
}

function customRuleTarget(rule: CustomRuleEntry, fallbackLabel: string): string {
  if (rule.target_mode === 'direct') return 'DIRECT';
  if (rule.target_mode === 'reject') return 'REJECT';
  if (rule.target_mode === 'existing_group') return rule.target_group || '🚀 节点选择';
  return rule.label || fallbackLabel;
}

export default function RuleSourceCard({ doc, onPatch }: RuleSourceCardProps) {
  const { t } = useTranslation();
  const messageApi = getMessage();

  const templateRuleMode =
    typeof doc.template_rule_mode === 'string' ? doc.template_rule_mode : 'rules';
  const ruleMode = typeof doc.rule_mode === 'string' ? doc.rule_mode : '';
  const enabledRaw = readStrings(doc.enabled_rules);
  const activeSet = useMemo(() => enabledRuleSet(ruleMode, enabledRaw), [ruleMode, enabledRaw]);

  const [expanded, setExpanded] = useState(false);
  const [draftText, setDraftText] = useState<string | null>(null);
  const [quickName, setQuickName] = useState('');
  const [quickUrls, setQuickUrls] = useState('');
  const [modal, setModal] = useState(false);
  const [editing, setEditing] = useState<CustomRuleDraft | null>(null);

  const customRules = readCustomRules(doc.custom_rules);
  const additionalRules = readStrings(doc.additional_rules);
  const directExternal = additionalRules.join('\n');
  const directValue = draftText ?? directExternal;

  const externalConfig =
    typeof doc.external_config === 'object' && doc.external_config !== null
      ? (doc.external_config as Record<string, unknown>)
      : {};
  const templateKey =
    typeof externalConfig.template_key === 'string' ? externalConfig.template_key : 'none';
  const templateValue = typeof doc.template === 'string' ? doc.template : 'standard';

  const chips = RULE_CATALOG;
  const visibleChips = expanded ? chips : chips.slice(0, VISIBLE_CHIPS);

  const orderedEnabled = (keys: Iterable<string>) => {
    const set = new Set(keys);
    return RULE_CATALOG_KEYS.filter((key) => set.has(key));
  };

  const toggleChip = (key: string) => {
    const next = new Set(activeSet);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    onPatch({ rule_mode: 'custom', enabled_rules: orderedEnabled(next) });
  };

  const applyPreset = (mode: string) => {
    if (mode === 'custom') {
      onPatch({ rule_mode: 'custom', enabled_rules: orderedEnabled(activeSet) });
      return;
    }
    onPatch({ rule_mode: mode === 'balanced' ? undefined : mode, enabled_rules: undefined });
  };

  const commitDirectRules = () => {
    if (draftText === null) return;
    const text = draftText;
    setDraftText(null);
    const normalized = normalizeDirectRuleLines(text);
    const next = normalized.length > 0 ? normalized : undefined;
    if ((next ?? []).join('\n') === directExternal) return;
    onPatch({ additional_rules: next });
  };

  const commitQuickAdd = () => {
    const { entries, invalid } = parseQuickAddLines(quickUrls, quickName);
    if (entries.length === 0) {
      messageApi.warning(t('pages.subscription.remoteRuleNothingToast'));
      return;
    }
    const taken = new Set([...RULE_CATALOG_KEYS, ...customRules.map((rule) => rule.key)]);
    const added = buildQuickAddRules(entries, taken);
    onPatch({ custom_rules: [...customRules, ...added] });
    const parts = [t('pages.subscription.remoteRuleAdded', { count: added.length })];
    if (invalid > 0) parts.push(t('pages.subscription.remoteRuleSkipped', { count: invalid }));
    messageApi.success(parts.join('，'));
    setQuickName('');
    setQuickUrls('');
  };

  const writeCustomRules = (rules: CustomRuleEntry[]) => onPatch({ custom_rules: rules });

  const submitCustomRule = (entry: CustomRuleEntry, index: number) => {
    const next = [...customRules];
    if (index >= 0) next[index] = entry;
    else next.push(entry);
    writeCustomRules(next);
    setModal(false);
    setEditing(null);
  };

  const sourceLabel = (rule: CustomRuleEntry) => {
    switch (rule.source_type) {
      case 'http':
        return t('pages.subscription.customSourceHttp');
      case 'file':
        return t('pages.subscription.customSourceFile');
      case 'group_only':
        return t('pages.subscription.customSourceGroupOnly');
      default:
        return t('pages.subscription.customSourceInline');
    }
  };

  const builtinPanel = (
    <div>
      <Space style={{ marginBottom: 10 }} wrap>
        <Typography.Text>{t('pages.subscription.rulePresets')}</Typography.Text>
        <Radio.Group
          value={ruleMode || 'balanced'}
          optionType="button"
          buttonStyle="solid"
          onChange={(e) => applyPreset(e.target.value as string)}
          options={[
            { value: 'minimal', label: t('pages.subscription.ruleModeMinimal') },
            { value: 'balanced', label: t('pages.subscription.ruleModeBalanced') },
            { value: 'full', label: t('pages.subscription.ruleModeFull') },
            { value: 'custom', label: t('pages.subscription.ruleModeCustom') },
          ]}
        />
        <Typography.Text type="secondary">
          {t('pages.subscription.enabledRuleCount', {
            enabled: activeSet.size,
            total: RULE_CATALOG_KEYS.length,
          })}
        </Typography.Text>
      </Space>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
        {visibleChips.map((entry) => {
          const selected = activeSet.has(entry.key);
          const label = t(`pages.subscription.ruleCatalog.${entry.key}`);
          return (
            <Button
              key={entry.key}
              size="small"
              type={selected ? 'primary' : 'default'}
              icon={entry.icon}
              aria-pressed={selected}
              aria-label={label}
              onClick={() => toggleChip(entry.key)}
            >
              {label}
            </Button>
          );
        })}
      </div>
      {chips.length > VISIBLE_CHIPS && (
        <div style={{ marginTop: 8 }}>
          <Button size="small" type="text" onClick={() => setExpanded((v) => !v)}>
            {expanded
              ? t('pages.subscription.collapseChips')
              : t('pages.subscription.expandAllChips')}
          </Button>
        </div>
      )}
    </div>
  );

  const customPanel = (
    <div>
      <Typography.Title level={5}>{t('pages.subscription.directRulesTitle')}</Typography.Title>
      <Typography.Text>{t('pages.subscription.directRulesLabel')}</Typography.Text>
      <Input.TextArea
        rows={8}
        style={{ marginTop: 4, marginBottom: 4 }}
        aria-label={t('pages.subscription.directRulesLabel')}
        value={directValue}
        onChange={(e) => setDraftText(e.target.value)}
        onBlur={commitDirectRules}
        placeholder={
          'DOMAIN-KEYWORD,tracker,PROXY\nIP-CIDR,10.0.0.6/32,PROXY\nDOMAIN-SUFFIX,pp.ua,DIRECT'
        }
      />
      <Typography.Text type="secondary">{t('pages.subscription.directRulesHelp')}</Typography.Text>

      <Typography.Title level={5} style={{ marginTop: 16 }}>
        {t('pages.subscription.remoteQuickAddTitle')}
      </Typography.Title>
      <Input
        aria-label={t('pages.subscription.remoteRuleNameLabel')}
        placeholder={t('pages.subscription.remoteRuleNamePlaceholder')}
        value={quickName}
        onChange={(e) => setQuickName(e.target.value)}
        style={{ marginBottom: 8 }}
      />
      <Input.TextArea
        rows={4}
        aria-label={t('pages.subscription.remoteRuleUrlsLabel')}
        placeholder={'https://example.com/rules/ai.mrs\nhttps://example.com/rules/social.mrs'}
        value={quickUrls}
        onChange={(e) => setQuickUrls(e.target.value)}
        style={{ marginBottom: 8 }}
      />
      <Space style={{ marginBottom: 4 }}>
        <Button type="primary" onClick={commitQuickAdd}>
          {t('pages.subscription.remoteRuleAddBtn')}
        </Button>
        <Typography.Text type="secondary">{t('pages.subscription.remoteRuleHelp')}</Typography.Text>
      </Space>

      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginTop: 16,
          marginBottom: 8,
        }}
      >
        <Typography.Title level={5} style={{ margin: 0 }}>
          {t('pages.subscription.customRulesTitle')}
        </Typography.Title>
        <Button
          size="small"
          icon={<PlusOutlined aria-hidden />}
          aria-label={t('pages.subscription.customRuleAddBtn')}
          onClick={() => {
            setEditing(null);
            setModal(true);
          }}
        >
          {t('pages.subscription.customRuleAddBtn')}
        </Button>
      </div>
      {customRules.length === 0 && (
        <Typography.Text type="secondary">
          {t('pages.subscription.customRulesEmpty')}
        </Typography.Text>
      )}
      {customRules.map((rule, index) => (
        <div
          key={rule.key || index}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 12,
            padding: '6px 0',
          }}
        >
          <div>
            <div>{rule.label || rule.key}</div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {`${sourceLabel(rule)} · ${rule.format || 'text'} → ${customRuleTarget(
                rule,
                rule.label,
              )}`}
            </Typography.Text>
          </div>
          <Space>
            <Switch
              size="small"
              aria-label={t('enabled')}
              checked={rule.enabled}
              onChange={(checked) =>
                writeCustomRules(
                  customRules.map((r, i) => (i === index ? { ...r, enabled: checked } : r)),
                )
              }
            />
            <Button
              size="small"
              aria-label={t('pages.subscription.customRuleEditTitle')}
              icon={<EditOutlined aria-hidden />}
              onClick={() => {
                setEditing({ index, entry: rule });
                setModal(true);
              }}
            />
            <Popconfirm
              title={t('pages.subscription.customRuleDeleteConfirm')}
              okText={t('delete')}
              cancelText={t('cancel')}
              onConfirm={() => writeCustomRules(customRules.filter((_, i) => i !== index))}
            >
              <Button
                size="small"
                danger
                aria-label={t('pages.subscription.customRuleRemoveBtn')}
                icon={<DeleteOutlined aria-hidden />}
              />
            </Popconfirm>
          </Space>
        </div>
      ))}
    </div>
  );

  const rulesPanel = (
    <Tabs
      items={[
        {
          key: 'builtin',
          label: t('pages.subscription.ruleSubtabBuiltin'),
          children: builtinPanel,
        },
        {
          key: 'custom',
          label: t('pages.subscription.ruleSubtabCustom'),
          children: customPanel,
        },
      ]}
    />
  );

  return (
    <Card
      size="small"
      style={{ marginBottom: 10 }}
      title={t('pages.subscription.ruleSourceSection')}
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 10 }}
        title={t('pages.subscription.ruleSourceIntro')}
      />
      <Radio.Group
        style={{ marginBottom: 12 }}
        optionType="button"
        buttonStyle="solid"
        value={templateRuleMode === 'template' ? 'template' : 'rules'}
        onChange={(e) =>
          onPatch({
            template_rule_mode: e.target.value === 'template' ? 'template' : undefined,
          })
        }
        options={[
          { value: 'rules', label: t('pages.subscription.ruleModeTab') },
          { value: 'template', label: t('pages.subscription.templateModeTab') },
        ]}
      />
      {templateRuleMode === 'template' ? (
        <Space direction="vertical" style={{ width: '100%' }} size={10}>
          <div>
            <div style={{ marginBottom: 4 }}>{t('pages.subscription.templateCatalogLabel')}</div>
            <Select
              style={{ minWidth: 280 }}
              virtual={false}
              aria-label={t('pages.subscription.templateCatalogLabel')}
              value={templateKey}
              onChange={(value: string) => {
                if (value === 'none') {
                  onPatch({ external_config: undefined });
                  return;
                }
                const preset = TEMPLATE_PRESETS.find((p) => p.key === value);
                onPatch({
                  external_config: { template_key: value, template_label: preset?.label ?? '' },
                });
              }}
              options={[
                {
                  value: 'none',
                  label: t('pages.subscription.templateCatalogNone'),
                },
                ...Object.entries(
                  TEMPLATE_PRESETS.reduce<Record<string, typeof TEMPLATE_PRESETS>>(
                    (groups, preset) => {
                      groups[preset.group] = [...(groups[preset.group] ?? []), preset];
                      return groups;
                    },
                    {},
                  ),
                ).map(([group, presets]) => ({
                  label: group,
                  options: presets.map((preset) => ({ value: preset.key, label: preset.label })),
                })),
              ]}
            />
          </div>
          <div>
            <div style={{ marginBottom: 4 }}>{t('pages.subscription.template')}</div>
            <Select
              style={{ minWidth: 200 }}
              aria-label={t('pages.subscription.template')}
              value={templateValue}
              onChange={(value: string) => onPatch({ template: value })}
              options={[
                { value: 'lite', label: t('pages.subscription.templateLite') },
                { value: 'standard', label: t('pages.subscription.templateStandard') },
                { value: 'full', label: t('pages.subscription.templateFull') },
              ]}
            />
          </div>
        </Space>
      ) : (
        rulesPanel
      )}
      <CustomRuleModal
        open={modal}
        editing={editing}
        takenKeys={customRules.map((rule) => rule.key)}
        onClose={() => {
          setModal(false);
          setEditing(null);
        }}
        onSubmit={submitCustomRule}
      />
    </Card>
  );
}
