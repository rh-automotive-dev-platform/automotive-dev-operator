import { useState, useRef, useCallback, useEffect, useMemo } from 'react';
import { usePolling, PollingMessage } from './usePolling';

export interface BuildItem {
  name: string;
  phase: string;
  message: string;
  requestedBy?: string;
  createdAt: string;
  startTime?: string;
  completionTime?: string;
}

export interface UseBuildsListOptions {
  onError?: (error: string) => void;
  onBuildCreated?: (build: BuildItem) => void;
  onBuildUpdated?: (build: BuildItem) => void;
  onBuildDeleted?: (build: BuildItem) => void;
}

export interface UseBuildsListReturn {
  builds: BuildItem[];
  isConnected: boolean;
  isConnecting: boolean;
  error: string | null;
  startStream: () => void;
  stopStream: () => void;
  refreshBuilds: () => void;
}

export const useBuildsList = (options: UseBuildsListOptions = {}): UseBuildsListReturn => {
  const { onError, onBuildCreated, onBuildUpdated, onBuildDeleted } = options;

  // Create stable refs for callbacks to avoid recreating useSSE hook
  const onErrorRef = useRef(onError);
  const onBuildCreatedRef = useRef(onBuildCreated);
  const onBuildUpdatedRef = useRef(onBuildUpdated);
  const onBuildDeletedRef = useRef(onBuildDeleted);

  // Update refs when callbacks change
  useEffect(() => {
    onErrorRef.current = onError;
    onBuildCreatedRef.current = onBuildCreated;
    onBuildUpdatedRef.current = onBuildUpdated;
    onBuildDeletedRef.current = onBuildDeleted;
  }, [onError, onBuildCreated, onBuildUpdated, onBuildDeleted]);

  const [builds, setBuilds] = useState<BuildItem[]>([]);
  const [pollingUrl, setPollingUrl] = useState<string | null>(null);

  const buildsRef = useRef<BuildItem[]>([]);

  const handleMessage = useCallback((message: PollingMessage) => {
    const { event, data, id } = message;

    console.log('useBuildsList: Received message:', { event, data, id });

    switch (event) {
      case 'connected':
        console.log('useBuildsList: Connected to builds stream');
        break;

      case 'initial-list':
        if (data) {
          try {
            const initialBuilds: BuildItem[] = JSON.parse(data);
            buildsRef.current = initialBuilds;
            setBuilds(initialBuilds);
            console.log('useBuildsList: Received initial builds list:', initialBuilds.length);
          } catch (err) {
            console.error('useBuildsList: Failed to parse initial builds list:', err);
            onErrorRef.current?.('Failed to parse initial builds list');
          }
        }
        break;

      case 'build-created':
        if (data) {
          try {
            const newBuild: BuildItem = JSON.parse(data);
            buildsRef.current = [...buildsRef.current, newBuild];
            setBuilds([...buildsRef.current]);
            onBuildCreatedRef.current?.(newBuild);
            console.log('useBuildsList: Build created:', newBuild.name);
          } catch (err) {
            console.error('useBuildsList: Failed to parse created build:', err);
          }
        }
        break;

      case 'build-updated':
        if (data) {
          try {
            const updatedBuild: BuildItem = JSON.parse(data);
            buildsRef.current = buildsRef.current.map(build => 
              build.name === updatedBuild.name ? updatedBuild : build
            );
            setBuilds([...buildsRef.current]);
            onBuildUpdatedRef.current?.(updatedBuild);
            console.log('useBuildsList: Build updated:', updatedBuild.name, updatedBuild.phase);
          } catch (err) {
            console.error('useBuildsList: Failed to parse updated build:', err);
          }
        }
        break;

      case 'build-deleted':
        if (data && id) {
          try {
            const deletedBuild: BuildItem = JSON.parse(data);
            buildsRef.current = buildsRef.current.filter(build => build.name !== id);
            setBuilds([...buildsRef.current]);
            onBuildDeletedRef.current?.(deletedBuild);
            console.log('useBuildsList: Build deleted:', id);
          } catch (err) {
            console.error('useBuildsList: Failed to parse deleted build:', err);
          }
        }
        break;

      case 'error':
        const errorMsg = data || 'Unknown error occurred';
        onErrorRef.current?.(errorMsg);
        console.error('useBuildsList: Error event:', errorMsg);
        break;

      case 'ping':
        // Server keepalive, no action needed
        break;

      case 'disconnected':
        console.log('useBuildsList: Stream disconnected');
        break;

      default:
        console.log('useBuildsList: Unknown event type:', event, data);
        break;
    }
  }, []);

  const handleError = useCallback((error: Error) => {
    console.error('useBuildsList: Polling error:', error);
    onErrorRef.current?.('Connection error');
  }, []);

  const handleOpen = useCallback(() => {
    console.log('useBuildsList: Polling connection opened');
  }, []);

  const handleClose = useCallback(() => {
    console.log('useBuildsList: Polling connection closed');
  }, []);

  const pollingOptions = useMemo(() => {
    console.log('useBuildsList: Creating new polling options object');
    return {
      onMessage: handleMessage,
      onError: handleError,
      onOpen: handleOpen,
      onClose: handleClose,
      pollInterval: 2000,
    };
  }, [handleMessage, handleError, handleOpen, handleClose]);

  const { isConnected, isConnecting, error } = usePolling(pollingUrl, pollingOptions);

  const startStream = useCallback(() => {
    console.log('useBuildsList: Starting builds polling');
    const newUrl = '/v1/builds';
    setPollingUrl(currentUrl => {
      if (currentUrl !== newUrl) {
        console.log('useBuildsList: Setting new polling URL:', newUrl);
        return newUrl;
      } else {
        console.log('useBuildsList: URL unchanged, not reconnecting');
        return currentUrl;
      }
    });
  }, []);

  const stopStream = useCallback(() => {
    console.log('useBuildsList: Stopping builds polling');
    setPollingUrl(null);
  }, []);

  const refreshBuilds = useCallback(async () => {
    console.log('useBuildsList: Refreshing builds via REST API');
    try {
      // Use plain fetch with credentials to match authFetch behavior
      const response = await fetch('/v1/builds', {
        credentials: 'include',
        cache: 'no-store',
      });
      if (response.ok) {
        const buildsData: BuildItem[] = await response.json();
        buildsRef.current = buildsData;
        setBuilds(buildsData);
      } else {
        throw new Error(`Failed to fetch builds: ${response.status}`);
      }
    } catch (err) {
      console.error('useBuildsList: Failed to refresh builds:', err);
      onErrorRef.current?.(`Failed to refresh builds: ${err}`);
    }
  }, []);

  return {
    builds,
    isConnected,
    isConnecting,
    error,
    startStream,
    stopStream,
    refreshBuilds,
  };
};
