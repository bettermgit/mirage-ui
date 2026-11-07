import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation } from 'react-router';
import {
  Alert,
  Button,
  Card,
  Col,
  ConfigProvider,
  FloatButton,
  Layout,
  Modal,
  Row,
  Space,
  Spin,
  message,
} from 'antd';

import { HttpUtil, PromiseUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useAllSettings } from '@/api/queries/useAllSettings';
import { AllSettingSchema } from '@/schemas/setting';
import AppSidebar from '@/layouts/AppSidebar';
import SubscriptionGeneralTab from '@/pages/settings/SubscriptionGeneralTab';
import SubscriptionFormatsTab from '@/pages/settings/SubscriptionFormatsTab';
import SubscriptionBalancersTab from '@/pages/settings/SubscriptionBalancersTab';
import MihomoEngineTab from './MihomoEngineTab';
import '@/pages/settings/SettingsPage.css';

interface ApiMsg {
  success?: boolean;
}

const tabSlugs = ['general', 'mihomo', 'formats', 'balancers'];

function scrollTarget() {
  return document.getElementById('content-layout') as HTMLElement;
}

export default function SubscriptionPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();

  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const { allSetting, updateSetting, fetched, spinning, setSpinning, saveDisabled, saveAll } =
    useAllSettings();

  const location = useLocation();
  const slug = location.hash.replace(/^#/, '');
  const activeSlug = tabSlugs.includes(slug) ? slug : 'general';

  async function onSave() {
    const result = AllSettingSchema.safeParse(allSetting);
    if (!result.success) {
      const issue = result.error.issues[0];
      const fieldPath = issue?.path.join('.') ?? 'value';
      const msgKey = issue?.message ?? 'somethingWentWrong';
      messageApi.error(`${fieldPath}: ${t(msgKey, { defaultValue: msgKey })}`);
      return;
    }
    await saveAll();
  }

  function restartPanel() {
    modal.confirm({
      title: t('pages.settings.restartPanel'),
      content: t('pages.settings.restartPanelDesc'),
      okText: t('pages.settings.restartPanel'),
      okButtonProps: { danger: true },
      cancelText: t('cancel'),
      onOk: async () => {
        setSpinning(true);
        try {
          const msg = (await HttpUtil.post('/panel/api/setting/restartPanel')) as ApiMsg;
          if (!msg?.success) return;
          await PromiseUtil.sleep(5000);
          window.location.reload();
        } finally {
          setSpinning(false);
        }
      },
    });
  }

  const pageClass = useMemo(() => {
    const classes = ['settings-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const categoryBody = useMemo(() => {
    switch (activeSlug) {
      case 'mihomo':
        return <MihomoEngineTab allSetting={allSetting} updateSetting={updateSetting} />;
      case 'formats':
        return <SubscriptionFormatsTab allSetting={allSetting} updateSetting={updateSetting} />;
      case 'balancers':
        return <SubscriptionBalancersTab allSetting={allSetting} updateSetting={updateSetting} />;
      default:
        return <SubscriptionGeneralTab allSetting={allSetting} updateSetting={updateSetting} />;
    }
  }, [activeSlug, allSetting, updateSetting]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin
              aria-live="off"
              spinning={spinning || !fetched}
              delay={200}
              description={t('loading')}
              size="large"
            >
              {!fetched ? (
                <div className="loading-spacer" />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 0 : 12]}>
                  <Col span={24}>
                    <Card hoverable>
                      <Row className="header-row">
                        <Col xs={24} sm={10} className="header-actions">
                          <Space>
                            <Button type="primary" disabled={saveDisabled} onClick={onSave}>
                              {t('pages.settings.save')}
                            </Button>
                            <Button
                              type="primary"
                              danger
                              disabled={!saveDisabled}
                              onClick={restartPanel}
                            >
                              {t('pages.settings.restartPanel')}
                            </Button>
                          </Space>
                        </Col>
                        <Col xs={24} sm={14} className="header-info">
                          <FloatButton.BackTop target={scrollTarget} visibilityHeight={200} />
                          <Alert
                            type="warning"
                            showIcon
                            title={t('pages.subscription.engineApplyHint')}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>
                  <Col span={24}>
                    <Card hoverable>{categoryBody}</Card>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
