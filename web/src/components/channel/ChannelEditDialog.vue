<template>
  <v-dialog v-model="open" max-width="560">
    <v-card class="pa-2" color="surface-light">
      <v-card-title class="text-h6 font-weight-bold pt-4">Customize channel</v-card-title>
      <v-card-text>
        <div class="d-flex align-center ga-5 mb-6">
          <v-avatar v-if="avatarPreview" size="96"><v-img :src="avatarPreview" cover /></v-avatar>
          <ChannelAvatar v-else :channel="profile" :size="96" />
          <div>
            <div class="text-body-2 text-muted mb-2">JPG or PNG, square works best.</div>
            <v-btn variant="tonal" prepend-icon="mdi-camera-outline" :loading="uploadingAvatar" @click="avatarInput?.click()">Change picture</v-btn>
            <input ref="avatarInput" type="file" :accept="IMAGE_ACCEPT" hidden @change="onAvatarSelected" />
          </div>
        </div>
        <v-form ref="formRef" @submit.prevent="save">
          <v-text-field
            v-model="channelName"
            label="Channel name"
            counter="60"
            :rules="[(v: string) => (v.trim().length >= 3 && v.trim().length <= 60) || 'Use 3–60 characters']"
            class="mb-2"
          />
          <v-textarea v-model="description" label="Description" rows="4" counter="1000" :rules="[(v: string) => v.length <= 1000 || 'Too long']" />
        </v-form>
      </v-card-text>
      <v-card-actions class="px-6 pb-4">
        <v-spacer />
        <v-btn variant="text" @click="open = false">Cancel</v-btn>
        <v-btn color="accent" variant="flat" :loading="isPending" :disabled="uploadingAvatar" @click="save">Save</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue";
import { push } from "notivue";
import type { VForm } from "vuetify/components";
import ChannelAvatar from "./ChannelAvatar.vue";
import { IMAGE_ACCEPT, isAcceptedImage, uploadObject } from "@/composables/file/uploadObject";
import { useUpdateProfile } from "@/composables/user/user.mutation";
import { errorMessage } from "@/api/http";
import type { Profile } from "@/types";

const open = defineModel<boolean>({ default: false });
const props = defineProps<{ profile: Profile }>();

const formRef = ref<InstanceType<typeof VForm>>();
const avatarInput = ref<HTMLInputElement>();
const channelName = ref("");
const description = ref("");
const avatarKey = ref<string>();
const avatarPreview = ref("");
const uploadingAvatar = ref(false);
const { mutate, isPending } = useUpdateProfile();

/** swaps the local preview, freeing the previous object URL */
const setPreview = (url: string) => {
  if (avatarPreview.value) URL.revokeObjectURL(avatarPreview.value);
  avatarPreview.value = url;
};
onBeforeUnmount(() => setPreview(""));

watch(
  open,
  (isOpen) => {
    if (!isOpen) return;
    channelName.value = props.profile.channel_name || props.profile.user_name;
    description.value = props.profile.description || "";
    avatarKey.value = undefined;
    setPreview("");
  },
  { immediate: true }
);

const onAvatarSelected = async (e: Event) => {
  const file = (e.target as HTMLInputElement).files?.[0];
  (e.target as HTMLInputElement).value = "";
  if (!file) return;
  if (!isAcceptedImage(file)) {
    push.error("Please choose a JPEG, PNG, WebP or GIF image");
    return;
  }
  if (file.size > 5 * 1024 * 1024) {
    push.error("Pictures must be under 5 MB");
    return;
  }
  setPreview(URL.createObjectURL(file));
  uploadingAvatar.value = true;
  try {
    avatarKey.value = await uploadObject(`avatars/${props.profile.id}`, file);
  } catch (err) {
    setPreview("");
    push.error(errorMessage(err, "Avatar upload failed"));
  } finally {
    uploadingAvatar.value = false;
  }
};

const save = async () => {
  const { valid } = (await formRef.value?.validate()) ?? { valid: false };
  if (!valid) return;
  mutate(
    { channel_name: channelName.value.trim(), description: description.value, avatar: avatarKey.value },
    {
      onSuccess: () => {
        push.success("Channel updated");
        open.value = false;
      },
    }
  );
};
</script>
