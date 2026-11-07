import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, Modal, Switch } from 'antd';
import { FormProvider, useForm } from 'react-hook-form';

import { FormField, rhfZodValidate } from '@/components/form/rhf';
import type { SubConvSource } from '@/generated/zod';
import { SourceFormSchema, type SourceFormValues } from './workbenchSchemas';

function initialState(source: SubConvSource | null): SourceFormValues {
  return {
    name: source?.name ?? '',
    url: source?.url ?? '',
    enable: source ? source.enable : true,
  };
}

interface SourceFormModalProps {
  open: boolean;
  source: SubConvSource | null;
  onClose: () => void;
  onConfirm: (values: SourceFormValues) => void;
}

export default function SourceFormModal({
  open,
  source,
  onClose,
  onConfirm,
}: SourceFormModalProps) {
  const { t } = useTranslation();
  const isEdit = source != null;
  const methods = useForm<SourceFormValues>({ defaultValues: initialState(source) });

  useEffect(() => {
    if (open) methods.reset(initialState(source));
  }, [open, source, methods]);

  return (
    <Modal
      open={open}
      title={
        isEdit
          ? `${t('edit')} ${t('pages.subscription.sourceTitle')}`
          : `+ ${t('pages.subscription.addSource')}`
      }
      okText={isEdit ? t('pages.clients.submitEdit') : t('create')}
      cancelText={t('close')}
      mask={{ closable: false }}
      width="560px"
      onOk={methods.handleSubmit(onConfirm)}
      onCancel={onClose}
    >
      <FormProvider {...methods}>
        <Form layout="vertical">
          <FormField
            label={t('pages.subscription.sourceName')}
            name="name"
            required
            rules={{ validate: rhfZodValidate(SourceFormSchema.shape.name) }}
          >
            <Input placeholder={t('pages.subscription.sourceNamePlaceholder')} />
          </FormField>
          <FormField
            label={t('pages.subscription.sourceUrl')}
            name="url"
            required
            tooltip={t('pages.subscription.sourceUrlDesc')}
            rules={{ validate: rhfZodValidate(SourceFormSchema.shape.url) }}
          >
            <Input placeholder="https://example.com/sub?token=..." />
          </FormField>
          <FormField label={t('enabled')} name="enable">
            <Switch />
          </FormField>
        </Form>
      </FormProvider>
    </Modal>
  );
}
