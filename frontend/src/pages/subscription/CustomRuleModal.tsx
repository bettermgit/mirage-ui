import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Modal, Radio, Switch } from 'antd';
import { FormProvider, useForm } from 'react-hook-form';

import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { CustomRuleFormSchema, type CustomRuleFormValues } from './workbenchSchemas';
import {
  RULE_CATALOG_KEYS,
  normalizeDirectRuleLines,
  slugRuleKey,
  type CustomRuleEntry,
} from './ruleCatalog';
import { useWatch } from 'react-hook-form';

const ENGINE_GROUPS = ['🚀 节点选择', '⚡ 自动选择', '🐟 漏网之鱼'];

export interface CustomRuleDraft {
  index: number;
  entry: CustomRuleEntry;
}

interface CustomRuleModalProps {
  open: boolean;
  editing: CustomRuleDraft | null;
  takenKeys: string[];
  onClose: () => void;
  onSubmit: (entry: CustomRuleEntry, index: number) => void;
}

function initialValues(editing: CustomRuleDraft | null): CustomRuleFormValues {
  const entry = editing?.entry;
  return {
    name: entry?.label ?? '',
    sourceType: (entry?.source_type as CustomRuleFormValues['sourceType']) ?? 'inline',
    url: entry?.url ?? '',
    payloadText: (entry?.payload ?? []).join('\n'),
    targetMode: (entry?.target_mode as CustomRuleFormValues['targetMode']) ?? 'new_group',
    targetGroup: entry?.target_group ?? ENGINE_GROUPS[0],
    interval: entry?.interval ?? 86400,
    noResolve: entry?.no_resolve ?? false,
  };
}

export default function CustomRuleModal({
  open,
  editing,
  takenKeys,
  onClose,
  onSubmit,
}: CustomRuleModalProps) {
  const { t } = useTranslation();
  const methods = useForm<CustomRuleFormValues>({ defaultValues: initialValues(editing) });
  const { getValues, reset } = methods;
  const sourceType = useWatch({ control: methods.control, name: 'sourceType' });
  const targetMode = useWatch({ control: methods.control, name: 'targetMode' });
  const name = useWatch({ control: methods.control, name: 'name' });

  useEffect(() => {
    if (open) reset(initialValues(editing));
  }, [open, editing, reset]);

  const keyPreview = useMemo(() => slugRuleKey(name), [name]);

  const submit = (values: CustomRuleFormValues) => {
    let key = editing ? editing.entry.key : slugRuleKey(values.name) || 'custom_rule';
    if (!editing) {
      const stem = key;
      const taken = new Set([...RULE_CATALOG_KEYS, ...takenKeys]);
      for (let n = 2; taken.has(key); n += 1) key = `${stem}_${n}`.slice(0, 64);
    }
    const entry: CustomRuleEntry = {
      key: editing ? editing.entry.key : key,
      label: values.name.trim(),
      icon: editing?.entry.icon ?? '',
      enabled: editing ? editing.entry.enabled : true,
      target_mode: values.targetMode,
      target_group: values.targetMode === 'existing_group' ? values.targetGroup : '',
      source_type: values.sourceType,
      behavior: editing?.entry.behavior ?? '',
      format: editing?.entry.format ?? '',
      url: values.sourceType === 'http' ? values.url.trim() : '',
      path: editing?.entry.path ?? '',
      interval: values.sourceType === 'http' ? values.interval : undefined,
      payload:
        values.sourceType === 'inline' ? normalizeDirectRuleLines(values.payloadText) : undefined,
      insert_position: 'before_match',
      no_resolve: values.noResolve,
    };
    onSubmit(entry, editing?.index ?? -1);
  };

  const urlValidator = (value: string | undefined) => {
    if (getValues('sourceType') !== 'http') return true;
    const url = String(value ?? '');
    return url.startsWith('http://') || url.startsWith('https://')
      ? true
      : t('pages.subscription.errUrlHttp');
  };

  return (
    <Modal
      open={open}
      title={
        editing
          ? t('pages.subscription.customRuleEditTitle')
          : t('pages.subscription.customRuleAddTitle')
      }
      okText={t('OK')}
      cancelText={t('cancel')}
      mask={{ closable: false }}
      width="560px"
      onOk={methods.handleSubmit(submit)}
      onCancel={onClose}
    >
      <FormProvider {...methods}>
        <Form layout="vertical">
          <FormField
            label={t('pages.subscription.customRuleFieldName')}
            name="name"
            required
            tooltip={keyPreview || undefined}
            rules={{ validate: rhfZodValidate(CustomRuleFormSchema.shape.name) }}
          >
            <Input placeholder={t('pages.subscription.customRuleFieldNamePlaceholder')} />
          </FormField>
          <FormField label={t('pages.subscription.customRuleFieldSource')} name="sourceType">
            <Radio.Group
              options={[
                { value: 'inline', label: t('pages.subscription.customSourceInline') },
                { value: 'http', label: t('pages.subscription.customSourceHttp') },
                { value: 'group_only', label: t('pages.subscription.customSourceGroupOnly') },
              ]}
              optionType="button"
            />
          </FormField>
          {sourceType === 'inline' && (
            <FormField
              label={t('pages.subscription.customRulePayloadLabel')}
              name="payloadText"
              tooltip={t('pages.subscription.directRulesHelp')}
            >
              <Input.TextArea rows={6} placeholder={'domain,ads.example.com\nnetwork,10.0.0.0/8'} />
            </FormField>
          )}
          {sourceType === 'http' && (
            <FormField
              label={t('pages.subscription.customRuleUrlLabel')}
              name="url"
              rules={{ validate: urlValidator }}
            >
              <Input placeholder="https://example.com/rules/ai.mrs" />
            </FormField>
          )}
          {sourceType === 'http' && (
            <FormField label={t('pages.subscription.customRuleIntervalLabel')} name="interval">
              <InputNumber min={600} step={3600} style={{ width: 160 }} />
            </FormField>
          )}
          <FormField label={t('pages.subscription.customRuleFieldTarget')} name="targetMode">
            <Radio.Group
              options={[
                { value: 'new_group', label: t('pages.subscription.targetNewGroup') },
                { value: 'direct', label: 'DIRECT' },
                { value: 'reject', label: 'REJECT' },
                { value: 'existing_group', label: t('pages.subscription.targetExistingGroup') },
              ]}
              optionType="button"
            />
          </FormField>
          {targetMode === 'existing_group' && (
            <FormField label={t('pages.subscription.customRuleTargetGroup')} name="targetGroup">
              <Radio.Group
                options={ENGINE_GROUPS.map((group) => ({ value: group, label: group }))}
              />
            </FormField>
          )}
          <FormField label={t('pages.subscription.customRuleNoResolveLabel')} name="noResolve">
            <Switch />
          </FormField>
        </Form>
      </FormProvider>
    </Modal>
  );
}
