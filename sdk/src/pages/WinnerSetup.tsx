import { useState, useEffect } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Typography,
} from '@mui/material';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import { getRealBackendUrl, buildServiceUrl } from '../utils/proxy';
import { useTranslation } from '../utils/i18n';

function CodeBlock({ children }: { children: React.ReactNode }) {
  return (
    <Box
      sx={{
        position: 'relative',
        bgcolor: 'grey.900',
        color: 'grey.100',
        p: 2,
        borderRadius: 2,
        fontFamily: 'monospace',
        fontSize: '0.9rem',
        whiteSpace: 'pre-wrap',
        border: '1px solid',
        borderColor: 'grey.800',
      }}
    >
      {children}
    </Box>
  );
}

export default function WinnerSetup() {
  const { t } = useTranslation();
  const [baseUrl, setBaseUrl] = useState('');
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let mounted = true;

    // First check if the host has provided a service-specific SDK URL via manifest/presence
    // @ts-ignore
    const serviceSdkUrl = window.__HRP_SDK_CONFIG__?.winnerproxy?.sdk_url;

    if (serviceSdkUrl) {
      if (mounted) {
        setBaseUrl(serviceSdkUrl.replace(/\/$/, ''));
      }
    } else {
      // Fallback to dynamic relay-based URL calculation
      getRealBackendUrl().then((url: string) => {
        if (mounted) {
          setBaseUrl(buildServiceUrl(url, '').replace(/\/$/, ''));
        }
      });
    }

    return () => {
      mounted = false;
    };
  }, []);

  const handleCopy = async () => {
    if (!baseUrl) return;
    await navigator.clipboard.writeText(baseUrl);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <Box sx={{ maxWidth: 800, mx: 'auto', p: 3, display: 'flex', flexDirection: 'column', gap: 3 }}>
      <Typography variant="h5" component="h1">
        {t('winnerproxy.setup.title')}
      </Typography>

      <Typography variant="body1">
        {t('winnerproxy.setup.description')}
      </Typography>

      <Card>
        <CardContent>
          <Typography variant="subtitle2" color="text.secondary" gutterBottom>
            {t('winnerproxy.setup.api_url')}
          </Typography>
          <CodeBlock>{baseUrl}</CodeBlock>
          <Button
            variant="contained"
            startIcon={<ContentCopyIcon />}
            onClick={handleCopy}
            sx={{ mt: 2 }}
          >
            {copied ? t('winnerproxy.setup.copied') : t('winnerproxy.setup.copy')}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <Typography variant="h6" sx={{ mb: 1 }}>
            {t('winnerproxy.setup.usage_title')}
          </Typography>
          <Box component="ol" sx={{ pl: 2, m: 0 }}>
            <li>
              <Typography variant="body2" sx={{ mb: 1 }}>
                {t('winnerproxy.setup.usage_step_1')}
              </Typography>
            </li>
            <li>
              <Typography variant="body2" sx={{ mb: 1 }}>
                {t('winnerproxy.setup.usage_step_2')}
              </Typography>
            </li>
          </Box>
        </CardContent>
      </Card>
    </Box>
  );
}
