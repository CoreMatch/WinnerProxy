import { useEffect } from 'react';
// @ts-ignore
import { useTranslation as useTranslationHost } from 'react-i18next';
// @ts-ignore
import i18nHost from '../../../../../i18n';
import zhCN from '../locales/zh-CN.json';
import enUS from '../locales/en-US.json';

const resources = {
  'zh-CN': zhCN,
  'en-US': enUS,
};

let initialized = false;

export function useTranslation() {
  const { t, i18n } = useTranslationHost();

  useEffect(() => {
    if (!initialized && i18nHost) {
      Object.entries(resources).forEach(([lng, res]) => {
        i18nHost.addResourceBundle(lng, 'translation', res, true, true);
      });
      initialized = true;
    }
  }, []);

  return { t, i18n };
}
