import { useQuery } from '@tanstack/react-query';
import { listFeatures, getFeature } from '../api/client';

export function useFeatures() {
  return useQuery({
    queryKey: ['features'],
    queryFn: listFeatures,
    refetchInterval: 30000, // Refresh every 30 seconds as fallback
  });
}

export function useFeature(id: string) {
  return useQuery({
    queryKey: ['feature', id],
    queryFn: () => getFeature(id),
    enabled: !!id,
  });
}