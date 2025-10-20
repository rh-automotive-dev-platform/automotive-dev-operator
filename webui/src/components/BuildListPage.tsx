import React, { useState, useEffect, useRef } from 'react';
import {
  PageSection,
  Title,
  Card,
  CardBody,
  Button,
  Badge,
  EmptyState,
  EmptyStateActions,
  EmptyStateBody,
  Spinner,
  Alert,
  Modal,
  ModalVariant,
  ModalBody,
  ModalFooter,
  CodeBlock,
  CodeBlockCode,
  Tabs,
  Tab,
  TabTitleText,
  Bullseye,
  Flex,
  FlexItem,
  ActionGroup,
  ExpandableSection,
  Content,
  Stack,
  StackItem
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { CubesIcon, DownloadIcon, EyeIcon, RedoIcon } from '@patternfly/react-icons';
import { useNavigate } from 'react-router-dom';
import { authFetch, API_BASE, BUILD_API_BASE } from '../utils/auth';
import { useLogStream } from '../hooks/useLogStream';
import { useBuildsList, BuildItem as SSEBuildItem } from '../hooks/useBuildsList';

interface BuildItem {
  name: string;
  phase: string;
  message: string;
  requestedBy?: string;
  createdAt: string;
  startTime?: string;
  completionTime?: string;
}

interface BuildDetails {
  name: string;
  phase: string;
  message: string;
  requestedBy?: string;
  artifactURL?: string;
  artifactFileName?: string;
  startTime?: string;
  completionTime?: string;
}

interface BuildParams {
  name: string;
  manifest?: string;
  manifestFileName?: string;
  distro?: string;
  target?: string;
  architecture?: string;
  exportFormat?: string;
  mode?: string;
  automotiveImageBuilder?: string;
  compression?: string;
}

const BuildListPage: React.FC = () => {
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);
  const [selectedBuild, setSelectedBuild] = useState<string | null>(null);
  const [buildDetails, setBuildDetails] = useState<BuildDetails | null>(null);
  const [buildParams, setBuildParams] = useState<BuildParams | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<string | number>(0);
  const [loadingDetails, setLoadingDetails] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const logContainerRef = useRef<HTMLDivElement | null>(null);
  const autoRefreshIntervalRef = useRef<NodeJS.Timeout | null>(null);
  const [nowTs, setNowTs] = useState<number>(Date.now());
  const liveTimerRef = useRef<NodeJS.Timeout | null>(null);
  const [downloadingArtifact, setDownloadingArtifact] = useState<string | null>(null);
  const downloadInProgressRef = useRef<string | null>(null);

  // Use the new SSE log streaming hook
  const {
    logs,
    currentStep,
    isStreaming,
    isConnected,
    logStreamError,
    startStream,
    stopStream,
    clearLogs,
  } = useLogStream({
    onLogUpdate: () => {
      setTimeout(() => {
        const el = logContainerRef.current;
        if (el) {
          el.scrollTop = el.scrollHeight;
        }
      }, 0);
    },
    onError: (error) => {
      setError(`Log streaming error: ${error}`);
    },
  });

  // Use the new SSE builds list hook
  const {
    builds,
    isConnected: buildsConnected,
    isConnecting: buildsConnecting,
    error: buildsError,
    startStream: startBuildsStream,
    stopStream: stopBuildsStream,
    refreshBuilds,
  } = useBuildsList({
    onError: (error) => {
      setError(`Builds streaming error: ${error}`);
    },
    onBuildCreated: (build) => {
      console.log('New build created:', build.name);
    },
    onBuildUpdated: (build) => {
      console.log('Build updated:', build.name, build.phase);
    },
    onBuildDeleted: (build) => {
      console.log('Build deleted:', build.name);
    },
  });

  const scrollLogsToBottom = () => {
    requestAnimationFrame(() => {
      const el = logContainerRef.current;
      if (el) {
        el.scrollTop = el.scrollHeight;
      }
    });
  };

  useEffect(() => {
    scrollLogsToBottom();
  }, [logs]);

  useEffect(() => {
    if (isModalOpen && activeTab === 1) {
      setTimeout(scrollLogsToBottom, 0);
    }
  }, [isModalOpen, activeTab]);

  useEffect(() => {
    return () => {
      if (autoRefreshIntervalRef.current) {
        clearInterval(autoRefreshIntervalRef.current);
      }
    };
  }, []);

  useEffect(() => {
    const anyRunning = builds.some(b => b.phase && b.phase.toLowerCase() === 'building');
    if (anyRunning && !liveTimerRef.current) {
      liveTimerRef.current = setInterval(() => setNowTs(Date.now()), 1000);
    } else if (!anyRunning && liveTimerRef.current) {
      clearInterval(liveTimerRef.current);
      liveTimerRef.current = null;
    }
    return () => {
      if (liveTimerRef.current) {
        clearInterval(liveTimerRef.current);
        liveTimerRef.current = null;
      }
    };
  }, [builds]);

  const formatDuration = (totalSeconds: number): string => {
    const sec = Math.max(0, Math.floor(totalSeconds));
    const hh = Math.floor(sec / 3600);
    const mm = Math.floor((sec % 3600) / 60);
    const ss = sec % 60;
    if (hh > 0) return `${hh}h ${mm}m ${ss}s`;
    return `${mm}m ${ss}s`;
  };



  const fetchBuildDetails = async (buildName: string): Promise<BuildDetails | null> => {
    try {
      setLoadingDetails(true);
      const response = await authFetch(`${API_BASE}/v1/builds/${buildName}`);
      if (response.ok) {
        const data = await response.json();
        setBuildDetails(data);
        return data;
      } else {
        throw new Error(`Failed to fetch build details: ${response.status}`);
      }
    } catch (err) {
      setError(`Error fetching build details: ${err}`);
      return null;
    } finally {
      setLoadingDetails(false);
    }
  };


  const fetchBuildParams = async (buildName: string) => {
    try {
      const resp = await authFetch(`${API_BASE}/v1/builds/${buildName}/template`);
      if (!resp.ok) return;
      const tpl = await resp.json();
      setBuildParams({
        name: tpl.name,
        manifest: tpl.manifest,
        manifestFileName: tpl.manifestFileName,
        distro: tpl.distro,
        target: tpl.target,
        architecture: tpl.architecture,
        exportFormat: tpl.exportFormat,
        mode: tpl.mode,
        automotiveImageBuilder: tpl.automotiveImageBuilder,
        compression: tpl.compression,
      });
    } catch (e) {
    }
  };

  const downloadArtifact = async (buildName: string) => {
    if (downloadInProgressRef.current === buildName) return;
    if (downloadingArtifact) return;

    try {
      downloadInProgressRef.current = buildName;
      setDownloadingArtifact(buildName);
      setError(null);

      let artifactFileName: string;
      if (buildDetails && buildDetails.artifactFileName && selectedBuild === buildName) {
        artifactFileName = buildDetails.artifactFileName;
      } else {
        const resp = await authFetch(`${API_BASE}/v1/builds/${buildName}`);
        if (!resp.ok) {
          throw new Error(`Failed to fetch build details: ${resp.status}`);
        }
        const details = await resp.json();
        artifactFileName = details.artifactFileName;
        if (!artifactFileName) {
          throw new Error('Artifact filename not available');
        }
      }

      const url = `${API_BASE}/v1/builds/${buildName}/artifact/${encodeURIComponent(artifactFileName)}`;
      window.location.href = url;

      setTimeout(() => {
        setDownloadingArtifact(null);
        downloadInProgressRef.current = null;
      }, 1500);
    } catch (err) {
      setError(`Error initiating download: ${err}`);
      setDownloadingArtifact(null);
      downloadInProgressRef.current = null;
    }
  };

  const [artifactItems, setArtifactItems] = useState<{ name: string; sizeBytes: string }[] | null>(null);
  const [loadingItems, setLoadingItems] = useState<boolean>(false);
  const [downloadingItem, setDownloadingItem] = useState<string | null>(null);
  const [expandedItem, setExpandedItem] = useState<string | null>(null);

  const fetchArtifactItems = async (buildName: string) => {
    try {
      setLoadingItems(true);
      setError(null);
      const resp = await authFetch(`${API_BASE}/v1/builds/${buildName}/artifacts`);
      if (!resp.ok) {
        setArtifactItems([]);
        return;
      }
      const data = await resp.json();
      setArtifactItems(Array.isArray(data.items) ? data.items : []);
    } catch (e: any) {
      setArtifactItems([]);
    } finally {
      setLoadingItems(false);
    }
  };

  const downloadArtifactItem = async (buildName: string, fileName: string) => {
    if (downloadingItem) return;
    setDownloadingItem(fileName);
    try {
      const url = `${API_BASE}/v1/builds/${buildName}/artifacts/${encodeURIComponent(fileName)}`;
      window.location.href = url;
    } finally {
      setTimeout(() => setDownloadingItem(null), 1200);
    }
  };

  const startAutoRefresh = (buildName: string) => {
    if (autoRefreshIntervalRef.current) {
      clearInterval(autoRefreshIntervalRef.current);
    }

    setAutoRefresh(true);
    autoRefreshIntervalRef.current = setInterval(async () => {
      const fresh = await fetchBuildDetails(buildName);
      if (fresh && (fresh.phase === 'Completed' || fresh.phase === 'Failed')) {
        stopAutoRefresh();
        return;
      }
      // SSE handles log streaming automatically, no need to manually fetch logs
    }, 5000);
  };

  const stopAutoRefresh = () => {
    if (autoRefreshIntervalRef.current) {
      clearInterval(autoRefreshIntervalRef.current);
      autoRefreshIntervalRef.current = null;
    }
    setAutoRefresh(false);
    stopStream();
  };

  const openBuildModal = (buildName: string) => {
    setSelectedBuild(buildName);
    setIsModalOpen(true);
    setActiveTab(0);
    setArtifactItems(null);
    fetchBuildDetails(buildName);
    fetchBuildParams(buildName);
    // Clear logs when opening modal
    clearLogs();
  };

  const applyBuildAsTemplate = async (buildName: string) => {
    try {
      const resp = await fetch(`/v1/builds/${buildName}/template`);
      if (!resp.ok) {
        throw new Error(`Failed to fetch template: ${resp.status}`);
      }
      const template = await resp.json();
      sessionStorage.setItem('aib-template', JSON.stringify(template));
      navigate('/create', { state: { template } });
    } catch (err) {
      setError(`Error using as template: ${err}`);
    }
  };

  const closeModal = () => {
    stopAutoRefresh();
    setIsModalOpen(false);
    setSelectedBuild(null);
    setBuildDetails(null);
    clearLogs();
    setActiveTab(0);
  };

  const getPhaseVariant = (phase: string): 'blue' | 'cyan' | 'green' | 'orange' | 'purple' | 'red' | 'grey' => {
    switch (phase.toLowerCase()) {
      case 'completed':
        return 'green';
      case 'failed':
        return 'red';
      case 'building':
        return 'blue';
      default:
        return 'grey';
    }
  };

  // Start SSE builds streaming on component mount
  useEffect(() => {
    startBuildsStream();
    return () => {
      stopBuildsStream();
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (buildsConnecting && builds.length === 0) {
    return (
      <PageSection>
        <Bullseye>
          <Spinner size="xl" />
        </Bullseye>
      </PageSection>
    );
  }

  return (
    <PageSection>
      <Flex justifyContent={{ default: 'justifyContentSpaceBetween' }} className="pf-v6-u-mb-lg">
        <FlexItem>
          <Title headingLevel="h1" size="2xl">
            Image Builds
          </Title>
        </FlexItem>
        <FlexItem>
          <Button variant="primary" onClick={() => navigate('/create')}>
            Create
          </Button>
          <Button
            variant="secondary"
            onClick={refreshBuilds}
            className="pf-v6-u-ml-sm"
            icon={<RedoIcon />}
          >
            Refresh
          </Button>
          {buildsConnecting && (
            <Badge color="blue" className="pf-v6-u-ml-sm">
              <Spinner size="sm" className="pf-v6-u-mr-xs" />
              Connecting...
            </Badge>
          )}
          {buildsError && (
            <Badge color="red" className="pf-v6-u-ml-sm">
              Connection Error
            </Badge>
          )}
        </FlexItem>
      </Flex>

      {error && (
        <Alert variant="danger" title={error} className="pf-v6-u-mb-lg" isInline />
      )}

      <Card>
        <CardBody>
          {builds.length === 0 ? (
            <EmptyState>
              <EmptyStateActions>
                <CubesIcon />
              </EmptyStateActions>
              <Title headingLevel="h4" size="lg">
                No builds found
              </Title>
              <EmptyStateBody>
                Create your first image build to get started.
              </EmptyStateBody>
              <EmptyStateActions>
                <Button variant="primary" onClick={() => navigate('/create')}>
                  Create Build
                </Button>
              </EmptyStateActions>
            </EmptyState>
          ) : (
            <Table aria-label="Builds table">
              <Thead>
                <Tr>
                  <Th>Name</Th>
                  <Th>Requested By</Th>
                  <Th>Status</Th>
                  <Th>Message</Th>
                  <Th>Created</Th>
                  <Th>Duration</Th>
                  <Th>Actions</Th>
                </Tr>
              </Thead>
              <Tbody>
                {builds.map((build) => (
                  <Tr key={build.name}>
                    <Td>{build.name}</Td>
                    <Td>{build.requestedBy || '-'}</Td>
                    <Td>
                      <Badge color={getPhaseVariant(build.phase)}>
                        {build.phase}
                      </Badge>
                    </Td>
                    <Td>{build.message}</Td>
                    <Td>{new Date(build.createdAt).toLocaleString()}</Td>
                    <Td>
                      {(() => {
                        const sRaw = ((build as any).startTime as string | undefined) || build.createdAt;
                        const eRaw = (build as any).completionTime as string | undefined;
                        const phase = (build.phase || '').toLowerCase();
                        const s = new Date(sRaw).getTime();
                        const ref = eRaw ? new Date(eRaw).getTime() : (phase === 'building' ? nowTs : NaN);
                        if (!isFinite(s) || !isFinite(ref) || ref < s) return '-';
                        return formatDuration(Math.round((ref - s) / 1000));
                      })()}
                    </Td>
                    <Td>
                      <Button
                        variant="link"
                        onClick={() => openBuildModal(build.name)}
                        icon={<EyeIcon />}
                      >
                        Details
                      </Button>
                      <Button
                        variant="secondary"
                        onClick={() => applyBuildAsTemplate(build.name)}
                        className="pf-v6-u-ml-sm"
                      >
                        Use as template
                      </Button>
                      {build.phase === 'Completed' && (
                        <Button
                          variant="secondary"
                          onClick={() => downloadArtifact(build.name)}
                          icon={<DownloadIcon />}
                          isLoading={downloadingArtifact === build.name}
                          isDisabled={!!downloadingArtifact}
                          className="pf-v6-u-ml-sm"
                        >
                          {downloadingArtifact === build.name ? 'Downloading...' : 'Download'}
                        </Button>
                      )}
                    </Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>
          )}
        </CardBody>
      </Card>

      <Modal
        variant={ModalVariant.large}
        title={`Build: ${selectedBuild}`}
        isOpen={isModalOpen}
        onClose={closeModal}
      >
        <ModalBody>
          <Tabs activeKey={activeTab} onSelect={(_event, tabIndex) => setActiveTab(tabIndex)}>
            <Tab eventKey={0} title={<TabTitleText>Details</TabTitleText>}>
              {loadingDetails ? (
                <Bullseye className="pf-v6-u-h-200">
                  <Spinner />
                </Bullseye>
              ) : buildDetails ? (
                <div className="pf-v6-u-py-md">
                  <Stack hasGutter>
                    <StackItem>
                      <Content><strong>Name:</strong> {buildDetails.name}</Content>
                    </StackItem>
                    <StackItem>
                      <Content><strong>Requested By:</strong> {buildDetails.requestedBy || '-'}</Content>
                    </StackItem>
                    <StackItem>
                      <Content>
                        <strong>Status:</strong> <Badge color={getPhaseVariant(buildDetails.phase)}>{buildDetails.phase}</Badge>
                      </Content>
                    </StackItem>
                    <StackItem>
                      <Content><strong>Message:</strong> {buildDetails.message}</Content>
                    </StackItem>
                    {buildDetails.startTime && (
                      <StackItem>
                        <Content><strong>Started:</strong> {new Date(buildDetails.startTime).toLocaleString()}</Content>
                      </StackItem>
                    )}
                    {buildDetails.completionTime && (
                      <>
                        <StackItem>
                          <Content><strong>Completed:</strong> {new Date(buildDetails.completionTime).toLocaleString()}</Content>
                        </StackItem>
                        <StackItem>
                          <Content><strong>Duration:</strong> {(() => {
                            const s = new Date(buildDetails.startTime || '').getTime();
                            const e = new Date(buildDetails.completionTime || '').getTime();
                            if (!isFinite(s) || !isFinite(e) || e < s) return '-';
                            const total = Math.floor((e - s) / 1000);
                            const hh = Math.floor(total / 3600);
                            const mm = Math.floor((total % 3600) / 60);
                            const ss = total % 60;
                            return hh > 0 ? `${hh}h ${mm}m ${ss}s` : `${mm}m ${ss}s`;
                          })()}</Content>
                        </StackItem>
                      </>
                    )}
                    {buildParams && (
                      <>
                        <StackItem>
                          <Content><strong>Distro:</strong> {buildParams.distro || '-'}</Content>
                        </StackItem>
                        <StackItem>
                          <Content><strong>Target:</strong> {buildParams.target || '-'}</Content>
                        </StackItem>
                        <StackItem>
                          <Content><strong>Architecture:</strong> {buildParams.architecture || '-'}</Content>
                        </StackItem>
                        <StackItem>
                          <Content><strong>Export format:</strong> {buildParams.exportFormat || '-'}</Content>
                        </StackItem>
                        <StackItem>
                          <Content><strong>Mode:</strong> {buildParams.mode || '-'}</Content>
                        </StackItem>
                        <StackItem>
                          <Content><strong>Image Builder:</strong> <span className="pf-v6-u-word-break-break-all">{buildParams.automotiveImageBuilder || '-'}</span></Content>
                        </StackItem>
                        <StackItem>
                          <Content><strong>Compression:</strong> {buildParams.compression || 'lz4'}</Content>
                        </StackItem>
                      </>
                    )}
                    {buildDetails.artifactURL && (
                      <StackItem>
                        <Content><strong>Artifact URL:</strong> {buildDetails.artifactURL}</Content>
                      </StackItem>
                    )}
                    {buildDetails.artifactFileName && (
                      <StackItem>
                        <Content><strong>Artifact File:</strong> {buildDetails.artifactFileName}</Content>
                      </StackItem>
                    )}
                  </Stack>

                  {buildDetails.phase === 'Completed' && selectedBuild && (
                    <div className="pf-v6-u-mt-lg">
                      <ActionGroup className="pf-v6-u-mb-md">
                        <Button
                          variant="secondary"
                          onClick={() => downloadArtifact(selectedBuild)}
                          icon={<DownloadIcon />}
                          isLoading={downloadingArtifact === selectedBuild}
                          isDisabled={!!downloadingArtifact}
                        >
                          {downloadingArtifact === selectedBuild ? 'Downloading...' :
                            (buildDetails.artifactFileName && (buildDetails.artifactFileName.includes('.tar.') || buildDetails.artifactFileName.includes('.zip')) ? 
                              'Download Complete Archive' : 'Download')}
                        </Button>
                        <Button
                          variant="tertiary"
                          onClick={() => fetchArtifactItems(selectedBuild)}
                          isLoading={loadingItems}
                          isDisabled={loadingItems}
                        >
                          {loadingItems ? 'Loading...' : 'Artifacts'}
                        </Button>
                      </ActionGroup>
                      {loadingItems && (
                        <div className="pf-v6-u-mb-sm">
                          <Spinner size="md" /> Loading items…
                        </div>
                      )}
                      {artifactItems && artifactItems.length > 0 && (
                        <div className="pf-v6-u-mb-md">
                          <Table aria-label="Artifact items table">
                            <Thead>
                              <Tr>
                                <Th>Item</Th>
                                <Th>Size (bytes)</Th>
                                <Th>Action</Th>
                              </Tr>
                            </Thead>
                            <Tbody>
                              {artifactItems.map((it) => (
                                <Tr key={it.name}>
                                  <Td>{it.name}</Td>
                                  <Td>{it.sizeBytes}</Td>
                                  <Td>
                                    <Button
                                      variant="secondary"
                                      onClick={() => downloadArtifactItem(selectedBuild, it.name)}
                                      isLoading={downloadingItem === it.name}
                                      isDisabled={!!downloadingItem}
                                    >
                                      {downloadingItem === it.name ? 'Downloading…' : 'Download'}
                                    </Button>
                                    <div className="pf-v6-u-mt-xs">
                                      <ExpandableSection
                                        toggleText={expandedItem === it.name ? 'Hide command' : 'Show command'}
                                        isExpanded={expandedItem === it.name}
                                        onToggle={() => setExpandedItem(expandedItem === it.name ? null : it.name)}
                                      >
                                        <div className="pf-v6-u-mb-xs">
                                          <CodeBlock>
                                            <CodeBlockCode>
{`GET ${BUILD_API_BASE || (API_BASE ? API_BASE : window.location.origin)}/v1/builds/${selectedBuild}/artifacts/${encodeURIComponent(it.name)}`}
                                            </CodeBlockCode>
                                          </CodeBlock>
                                        </div>
                                        <div>
                                          <p className="pf-v6-u-mt-0 pf-v6-u-mb-xs">Example with curl:</p>
                                          <CodeBlock>
                                            <CodeBlockCode>
{`TOKEN=$(oc whoami -t)
curl -H "Authorization: Bearer $TOKEN" \
     -OJ \
     "${BUILD_API_BASE || (API_BASE ? API_BASE : window.location.origin)}/v1/builds/${selectedBuild}/artifacts/${encodeURIComponent(it.name)}"`}
                                            </CodeBlockCode>
                                          </CodeBlock>
                                        </div>
                                      </ExpandableSection>
                                    </div>
                                  </Td>
                                </Tr>
                              ))}
                            </Tbody>
                          </Table>
                        </div>
                      )}
                      <Alert
                        variant="info"
                        title={buildDetails?.artifactFileName && (buildDetails.artifactFileName.includes('.tar.') || buildDetails.artifactFileName.includes('.zip')) ? 
                          `Direct Download - Complete Archive: ${buildDetails.artifactFileName}` :
                          `Direct Download${buildDetails?.artifactFileName ? `: ${buildDetails.artifactFileName}` : ''}`}
                        isInline
                        isPlain
                      >
                        {buildDetails?.artifactFileName && (
                          <>
                            {(buildDetails.artifactFileName.includes('.tar.') || buildDetails.artifactFileName.includes('.zip')) && (
                              <p className="pf-v6-u-mb-xs pf-v6-u-font-weight-bold">
                                This is a packaged archive containing all build artifacts. Use the "Artifacts" button above to download individual parts.
                              </p>
                            )}
                            <p className="pf-v6-u-mb-xs">
                              Direct file URL:
                            </p>
                            <CodeBlock>
                              <CodeBlockCode>
{`GET ${(buildDetails.artifactURL || API_BASE || window.location.origin).replace(/\/$/, '')}/v1/builds/${selectedBuild}/artifact/${buildDetails.artifactFileName}`}
                              </CodeBlockCode>
                            </CodeBlock>
                            <p className="pf-v6-u-mt-xs pf-v6-u-mb-xs">
                              Example with curl:
                            </p>
                            <CodeBlock>
                              <CodeBlockCode>
{`TOKEN=$(oc whoami -t)
curl -H "Authorization: Bearer $TOKEN" \\
     -OJ \\
     "${(buildDetails.artifactURL || API_BASE || window.location.origin).replace(/\/$/, '')}/v1/builds/${selectedBuild}/artifact/${encodeURIComponent(buildDetails.artifactFileName)}"`}
                              </CodeBlockCode>
                            </CodeBlock>
                          </>
                        )}

                      </Alert>
                    </div>
                  )}
                </div>
              ) : null}
            </Tab>
            <Tab
              eventKey={1}
              title={<TabTitleText>Logs</TabTitleText>}
            >
              <div className="pf-v6-u-py-md">
                <Flex className="pf-v6-u-mb-md">
                  <FlexItem>
                    <Button
                      variant="secondary"
                      onClick={() => selectedBuild && startStream(selectedBuild)}
                      icon={<RedoIcon />}
                      isDisabled={isStreaming || isConnected}
                    >
                      {isStreaming ? 'Streaming...' : isConnected ? 'Connected' : 'Start Log Stream'}
                    </Button>
                    {(isStreaming || isConnected) && (
                      <Button
                        variant="tertiary"
                        onClick={stopStream}
                        className="pf-v6-u-ml-sm"
                      >
                        Stop Stream
                      </Button>
                    )}
                  </FlexItem>
                  <FlexItem>
                    {isStreaming && (
                      <Badge isRead>
                        <Spinner size="sm" className="pf-v6-u-mr-xs" />
                        Streaming...
                      </Badge>
                    )}
                    {isConnected && (
                      <Badge className="pf-v6-u-ml-sm" color="green">
                        Connected
                      </Badge>
                    )}
                    {logStreamError && (
                      <Badge className="pf-v6-u-ml-sm" color="red">
                        Error: {logStreamError}
                      </Badge>
                    )}
                    {autoRefresh && (
                      <Badge className="pf-v6-u-ml-sm">
                        Auto-refresh enabled
                      </Badge>
                    )}
                    {currentStep && (
                      <Badge className="pf-v6-u-ml-sm" color="blue">
                        Step: {currentStep}
                      </Badge>
                    )}
                  </FlexItem>
                </Flex>
                <div
                  ref={logContainerRef}
                  className="pf-v6-u-max-height-60vh pf-v6-u-overflow-y-auto pf-v6-u-border-width-sm pf-v6-u-border-color-200 pf-v6-u-border-radius-sm"
                >
                  <CodeBlock>
                    <CodeBlockCode>
                      {logs || 'No logs available. Click "Start Log Stream" to begin streaming logs.'}
                    </CodeBlockCode>
                  </CodeBlock>
                </div>
              </div>
            </Tab>
          </Tabs>
        </ModalBody>
        <ModalFooter>
          <Button variant="primary" onClick={closeModal}>
            Close
          </Button>
        </ModalFooter>
      </Modal>
    </PageSection>
  );
};

export default BuildListPage;