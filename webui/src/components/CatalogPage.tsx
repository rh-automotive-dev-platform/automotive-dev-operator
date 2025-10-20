import React, { useState, useEffect } from 'react';
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
  Bullseye,
  Flex,
  FlexItem,
  Gallery,
  GalleryItem,
  Divider,
  Split,
  SplitItem,
  Stack,
  StackItem,
  Label,
  Content
} from '@patternfly/react-core';
import {
  DownloadIcon,
  CubesIcon,
  TagIcon,
  CalendarAltIcon,
  UserIcon
} from '@patternfly/react-icons';
import { useNavigate } from 'react-router-dom';
// import { authFetch, API_BASE } from '../utils/auth';

interface CatalogItem {
  id: string;
  name: string;
  description?: string;
  version: string;
  tags: string[];
  author?: string;
  createdAt: string;
  downloadCount: number;
  size?: string;
  architecture: string;
  distro: string;
  artifactURL?: string;
  templateURL?: string;
  thumbnailURL?: string;
}

const CatalogPage: React.FC = () => {
  const navigate = useNavigate();
  const [catalogItems, setCatalogItems] = useState<CatalogItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedItem, setSelectedItem] = useState<CatalogItem | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [downloadingItem, setDownloadingItem] = useState<string | null>(null);


  const fetchCatalogItems = async () => {
    try {
      setLoading(true);
      setError(null);

      // TODO: Replace with actual API call when catalog API is implemented
      // const response = await authFetch(`${API_BASE}/catalog`);
      // const data = await response.json();
      // setCatalogItems(data.items || []);

      // For now, show empty catalog
      setCatalogItems([]);

    } catch (err) {
      console.error('Error fetching catalog items:', err);
      setError('Failed to load catalog items');
      setCatalogItems([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchCatalogItems();
  }, []);

  const handleDownload = async (item: CatalogItem) => {
    try {
      setDownloadingItem(item.id);

      // TODO: Implement actual download logic
      console.log('Downloading item:', item.name);

      // Simulate download delay
      setTimeout(() => {
        setDownloadingItem(null);
        // TODO: Trigger actual download
      }, 2000);

    } catch (err) {
      console.error('Error downloading item:', err);
      setDownloadingItem(null);
    }
  };

  const handleUseAsTemplate = (item: CatalogItem) => {
    // Navigate to create build page with this item as a template
    navigate('/create', {
      state: {
        template: {
          name: item.name,
          distro: item.distro,
          architecture: item.architecture,
          templateId: item.id
        }
      }
    });
  };

  const openItemDetails = (item: CatalogItem) => {
    setSelectedItem(item);
    setIsModalOpen(true);
  };

  const closeModal = () => {
    setIsModalOpen(false);
    setSelectedItem(null);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    });
  };

  const getBadgeColor = (tag: string): "grey" | "red" | "blue" | "purple" | "green" | "orange" | "teal" | "orangered" | "yellow" | undefined => {
    const colorMap: { [key: string]: "grey" | "red" | "blue" | "purple" | "green" | "orange" | "teal" | "orangered" | "yellow" } = {
      'rhel': 'red',
      'centos': 'blue',
      'fedora': 'purple',
      'development': 'green',
      'base': 'grey',
      'iot': 'orange',
      'automotive': 'teal'
    };
    return colorMap[tag] || 'grey';
  };

  if (loading) {
    return (
      <PageSection>
        <Bullseye>
          <Spinner size="xl" />
        </Bullseye>
      </PageSection>
    );
  }

  if (error) {
    return (
      <PageSection>
        <Alert variant="danger" title="Error loading catalog">
          {error}
        </Alert>
      </PageSection>
    );
  }

  return (
    <>
      <PageSection>
        <Stack hasGutter>
          <StackItem>
            <Title headingLevel="h1" size="2xl">
              <CubesIcon className="pf-v6-u-mr-sm" />
              Image Catalog
            </Title>
          </StackItem>
          <StackItem>
            <Content component="p">
              Browse and download published image builds to use as templates for your projects.
            </Content>
          </StackItem>
        </Stack>
      </PageSection>

      <PageSection>
        {catalogItems.length === 0 ? (
          <EmptyState>
            <CubesIcon />
            <Title headingLevel="h4" size="lg">
              No catalog items found
            </Title>
            <EmptyStateBody>
              There are no published image builds available in the catalog yet.
            </EmptyStateBody>
            <EmptyStateActions>
              <Button variant="primary" onClick={() => navigate('/create')}>
                Create New Build
              </Button>
            </EmptyStateActions>
          </EmptyState>
        ) : (
          <Gallery hasGutter minWidths={{ default: '300px' }} maxWidths={{ default: '400px' }}>
            {catalogItems.map((item) => (
              <GalleryItem key={item.id}>
                <Card 
                  onClick={() => openItemDetails(item)}
                  className="pf-m-clickable"
                >
                  <CardBody>
                    <Stack hasGutter>
                      <StackItem>
                        <Split hasGutter>
                          <SplitItem isFilled>
                            <Title headingLevel="h3" size="lg">
                              {item.name}
                            </Title>
                          </SplitItem>
                          <SplitItem>
                            <Badge>{item.version}</Badge>
                          </SplitItem>
                        </Split>
                      </StackItem>

                      <StackItem>
                        <Content component="small">
                          {item.description || 'No description available'}
                        </Content>
                      </StackItem>

                      <StackItem>
                        <Flex spaceItems={{ default: 'spaceItemsXs' }}>
                          {item.tags.map((tag) => (
                            <FlexItem key={tag}>
                              <Label color={getBadgeColor(tag)} icon={<TagIcon />}>
                                {tag}
                              </Label>
                            </FlexItem>
                          ))}
                        </Flex>
                      </StackItem>

                      <StackItem>
                        <Split>
                          <SplitItem>
                            <Content component="small">
                              <CalendarAltIcon className="pf-v6-u-mr-xs" />
                              {formatDate(item.createdAt)}
                            </Content>
                          </SplitItem>
                          <SplitItem isFilled />
                          <SplitItem>
                            <Content component="small">
                              {item.downloadCount} downloads
                            </Content>
                          </SplitItem>
                        </Split>
                      </StackItem>

                      <StackItem>
                        <Split>
                          <SplitItem>
                            <Content component="small">
                              <strong>{item.architecture}</strong>
                            </Content>
                          </SplitItem>
                          <SplitItem isFilled />
                          <SplitItem>
                            <Content component="small">
                              {item.size}
                            </Content>
                          </SplitItem>
                        </Split>
                      </StackItem>

                      <Divider />

                      <StackItem>
                        <Flex>
                          <FlexItem>
                            <Button
                              variant="primary"
                              size="sm"
                              onClick={(e) => {
                                e.stopPropagation();
                                handleUseAsTemplate(item);
                              }}
                            >
                              Use as Template
                            </Button>
                          </FlexItem>
                          <FlexItem>
                            <Button
                              variant="secondary"
                              size="sm"
                              icon={<DownloadIcon />}
                              isLoading={downloadingItem === item.id}
                              onClick={(e) => {
                                e.stopPropagation();
                                handleDownload(item);
                              }}
                            >
                              Download
                            </Button>
                          </FlexItem>
                        </Flex>
                      </StackItem>
                    </Stack>
                  </CardBody>
                </Card>
              </GalleryItem>
            ))}
          </Gallery>
        )}
      </PageSection>

      {/* Item Details Modal */}
      <Modal
        variant={ModalVariant.medium}
        title={selectedItem?.name || ''}
        isOpen={isModalOpen}
        onClose={closeModal}
      >
        <ModalBody>
          {selectedItem && (
            <Stack hasGutter>
              <StackItem>
                <Content component="p">
                  {selectedItem.description || 'No description available'}
                </Content>
              </StackItem>

              <StackItem>
                <Split hasGutter>
                  <SplitItem>
                    <strong>Version:</strong> {selectedItem.version}
                  </SplitItem>
                  <SplitItem>
                    <strong>Architecture:</strong> {selectedItem.architecture}
                  </SplitItem>
                  <SplitItem>
                    <strong>Distribution:</strong> {selectedItem.distro}
                  </SplitItem>
                </Split>
              </StackItem>

              <StackItem>
                <Split hasGutter>
                  <SplitItem>
                    <strong>Size:</strong> {selectedItem.size}
                  </SplitItem>
                  <SplitItem>
                    <strong>Downloads:</strong> {selectedItem.downloadCount}
                  </SplitItem>
                  <SplitItem>
                    <strong>Created:</strong> {formatDate(selectedItem.createdAt)}
                  </SplitItem>
                </Split>
              </StackItem>

              {selectedItem.author && (
                <StackItem>
                  <Content>
                    <UserIcon className="pf-v6-u-mr-xs" />
                    <strong>Author:</strong> {selectedItem.author}
                  </Content>
                </StackItem>
              )}

              <StackItem>
                <Content><strong>Tags:</strong></Content>
                <Flex spaceItems={{ default: 'spaceItemsXs' }} className="pf-v6-u-mt-xs">
                  {selectedItem.tags.map((tag) => (
                    <FlexItem key={tag}>
                      <Label color={getBadgeColor(tag)} icon={<TagIcon />}>
                        {tag}
                      </Label>
                    </FlexItem>
                  ))}
                </Flex>
              </StackItem>
            </Stack>
          )}
        </ModalBody>
        <ModalFooter>
          <Button
            variant="primary"
            onClick={() => {
              if (selectedItem) {
                handleUseAsTemplate(selectedItem);
                closeModal();
              }
            }}
          >
            Use as Template
          </Button>
          <Button
            variant="secondary"
            icon={<DownloadIcon />}
            isLoading={downloadingItem === selectedItem?.id}
            onClick={() => {
              if (selectedItem) {
                handleDownload(selectedItem);
              }
            }}
          >
            Download
          </Button>
          <Button variant="link" onClick={closeModal}>
            Close
          </Button>
        </ModalFooter>
      </Modal>
    </>
  );
};

export default CatalogPage;
