import api from './index';

// 议题投票：作者发起/终止，读者投票，评论可选公开投票项。
export const pollApi = {
  create(data: {
    target_kind: string;
    target_id: number;
    question: string;
    description?: string;
    multiple?: boolean;
    anonymous?: boolean;
    options: string[];
  }) {
    return api.post('/polls', data);
  },
  get(id: number) {
    return api.get(`/polls/${id}`);
  },
  list(targetKind: string, targetId: number) {
    return api.get('/polls', { params: { target_kind: targetKind, target_id: targetId } });
  },
  vote(id: number, optionIds: number[]) {
    return api.post(`/polls/${id}/vote`, { option_ids: optionIds });
  },
  close(id: number) {
    return api.post(`/polls/${id}/close`);
  },
};
