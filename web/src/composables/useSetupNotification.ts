import { createNotivue } from "notivue";
import "notivue/notifications.css";
import "notivue/animations.css";
import "notivue/notification-progress.css";
export const useSetupNotification = () => {
  return createNotivue({
    pauseOnHover: true,
    position: "bottom-right",
    notifications: {
      global: {},
    },
  });
};
