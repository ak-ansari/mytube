<template>
  <section class="comments">
    <h2 class="text-h6 font-weight-bold mb-6">{{ count }} {{ count === 1 ? "Comment" : "Comments" }}</h2>

    <div class="d-flex ga-4 mb-8">
      <ChannelAvatar :channel="me" :size="40" />
      <div class="flex-1-1">
        <v-textarea
          v-model="draft"
          placeholder="Add a comment..."
          variant="underlined"
          density="compact"
          rows="1"
          auto-grow
          hide-details
          color="secondary"
          :maxlength="2000"
          @focus="onFocus"
        />
        <div v-if="composing" class="d-flex justify-end ga-2 mt-2">
          <v-btn variant="text" @click="cancel">Cancel</v-btn>
          <v-btn color="accent" variant="flat" :disabled="!draft.trim()" :loading="adding" @click="submit">Comment</v-btn>
        </div>
      </div>
    </div>

    <div v-for="c in items" :key="c.id" class="d-flex ga-4 mb-6 comment">
      <RouterLink :to="{ name: 'Channel', params: { channelId: c.author.id } }">
        <ChannelAvatar :channel="c.author" :size="40" />
      </RouterLink>
      <div class="flex-1-1 min-w-0">
        <div class="text-caption mb-1">
          <RouterLink :to="{ name: 'Channel', params: { channelId: c.author.id } }" class="font-weight-medium">@{{ c.author.user_name }}</RouterLink>
          <span v-if="c.author.id === ownerId" class="owner-badge ml-1">creator</span>
          <span class="text-muted ml-1">{{ timeAgo(c.created_at) }}</span>
        </div>
        <div class="text-body-2 content">{{ c.content }}</div>
      </div>
      <v-menu v-if="canDelete(c)" location="bottom end">
        <template #activator="{ props }">
          <v-btn v-bind="props" icon variant="text" size="small" class="comment-menu" aria-label="Comment actions">
            <v-icon>mdi-dots-vertical</v-icon>
          </v-btn>
        </template>
        <v-list density="compact" bg-color="surface-light">
          <v-list-item prepend-icon="mdi-delete-outline" title="Delete" @click="remove(c.id)" />
        </v-list>
      </v-menu>
    </div>

    <LoadMore :has-more="!!hasNextPage" :loading="isFetchingNextPage || isLoading" @load="fetchNextPage()" />
  </section>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { push } from "notivue";
import ChannelAvatar from "@/components/channel/ChannelAvatar.vue";
import LoadMore from "@/components/common/LoadMore.vue";
import { isAuthenticated, login } from "@/auth/keycloak";
import { useComments } from "@/composables/catalog/catalog.query";
import { useAddComment, useDeleteComment } from "@/composables/catalog/catalog.mutation";
import { useMe } from "@/composables/user/user.query";
import type { Comment } from "@/types";
import { timeAgo } from "@/utils";

const props = defineProps<{ videoId: string; ownerId: string; count: number }>();

const { data: me } = useMe();
const { items, isLoading, hasNextPage, isFetchingNextPage, fetchNextPage } = useComments(() => props.videoId);
const { mutate: addComment, isPending: adding } = useAddComment(() => props.videoId);
const { mutate: deleteComment } = useDeleteComment(() => props.videoId);

const draft = ref("");
const composing = ref(false);

const onFocus = (e: FocusEvent) => {
  if (!isAuthenticated.value) {
    (e.target as HTMLElement).blur();
    login();
    return;
  }
  composing.value = true;
};
const cancel = () => {
  draft.value = "";
  composing.value = false;
};
const submit = () =>
  addComment(draft.value.trim(), {
    onSuccess: cancel,
  });

const canDelete = (c: Comment) => !!me.value && (c.author.id === me.value.id || props.ownerId === me.value.id);
const remove = (id: string) => deleteComment(id, { onSuccess: () => push.success("Comment deleted") });
</script>

<style scoped>
.content {
  white-space: pre-wrap;
  word-break: break-word;
}
.owner-badge {
  font-size: 11px;
  padding: 0 6px;
  border-radius: 999px;
  background: rgb(var(--v-theme-surface-bright));
}
.comment-menu {
  opacity: 0;
}
.comment:hover .comment-menu {
  opacity: 1;
}
.min-w-0 {
  min-width: 0;
}
</style>
