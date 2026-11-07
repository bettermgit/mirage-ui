import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, Modal, Select, Switch } from 'antd';
import { FormProvider, useForm } from 'react-hook-form';

import { selectTagRender } from '@/components/form';
import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { useWorkbenchClientsQuery } from '@/api/queries/useSubConvWorkbench';
import type { SubConvPublication, SubConvSource } from '@/generated/zod';
import { PublicationFormSchema, type PublicationFormValues } from './workbenchSchemas';

function initialState(publication: SubConvPublication | null): PublicationFormValues {
  return {
    name: publication?.name ?? '',
    sourceIds: [...(publication?.sourceIds ?? [])],
    subIds: [...(publication?.subIds ?? [])],
    enable: publication ? publication.enable : true,
    rotateToken: false,
  };
}

interface PublicationFormModalProps {
  open: boolean;
  publication: SubConvPublication | null;
  sources: SubConvSource[];
  onClose: () => void;
  onConfirm: (values: PublicationFormValues) => void;
}

export default function PublicationFormModal({
  open,
  publication,
  sources,
  onClose,
  onConfirm,
}: PublicationFormModalProps) {
  const { t } = useTranslation();
  const isEdit = publication != null;
  const { data: clients } = useWorkbenchClientsQuery(open);
  const methods = useForm<PublicationFormValues>({ defaultValues: initialState(publication) });

  useEffect(() => {
    if (open) methods.reset(initialState(publication));
  }, [open, publication, methods]);

  const sourceOptions = useMemo(
    () =>
      sources.map((s) => ({
        label: s.enable ? s.name : `${s.name} (${t('disabled')})`,
        value: s.id,
      })),
    [sources, t],
  );
  const clientOptions = useMemo(
    () => (clients ?? []).map((c) => ({ label: `${c.email} (${c.subId})`, value: c.subId })),
    [clients],
  );

  function onFinish(values: PublicationFormValues) {
    if (values.sourceIds.length === 0 && values.subIds.length === 0) {
      methods.setError('subIds', {
        type: 'manual',
        message: t('pages.subscription.errSelectionRequired'),
      });
      return;
    }
    onConfirm(values);
  }

  return (
    <Modal
      open={open}
      title={
        isEdit
          ? `${t('edit')} ${t('pages.subscription.publicationTitle')}`
          : `+ ${t('pages.subscription.addPublication')}`
      }
      okText={isEdit ? t('pages.clients.submitEdit') : t('create')}
      cancelText={t('close')}
      mask={{ closable: false }}
      width="640px"
      onOk={methods.handleSubmit(onFinish)}
      onCancel={onClose}
    >
      <FormProvider {...methods}>
        <Form layout="vertical">
          <FormField
            label={t('pages.subscription.sourceName')}
            name="name"
            required
            rules={{ validate: rhfZodValidate(PublicationFormSchema.shape.name) }}
          >
            <Input placeholder={t('pages.subscription.publicationNamePlaceholder')} />
          </FormField>
          <FormField label={t('pages.subscription.pubSources')} name="sourceIds">
            <Select
              mode="multiple"
              allowClear
              tagRender={selectTagRender}
              placeholder={t('pages.subscription.pubSourcesPlaceholder')}
              options={sourceOptions}
            />
          </FormField>
          <FormField
            label={t('pages.subscription.pubClients')}
            name="subIds"
            tooltip={t('pages.subscription.pubClientsDesc')}
          >
            <Select
              mode="multiple"
              allowClear
              showSearch
              tagRender={selectTagRender}
              optionFilterProp="label"
              placeholder={t('pages.subscription.pubClientsPlaceholder')}
              options={clientOptions}
            />
          </FormField>
          <FormField label={t('enabled')} name="enable">
            <Switch />
          </FormField>
          {isEdit && (
            <FormField
              label={t('pages.subscription.rotateToken')}
              name="rotateToken"
              tooltip={t('pages.subscription.rotateTokenDesc')}
            >
              <Switch />
            </FormField>
          )}
        </Form>
      </FormProvider>
    </Modal>
  );
}
