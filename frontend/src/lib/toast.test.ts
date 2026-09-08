import { describe, expect, it, beforeEach } from "bun:test";
import { toast, toasts, clearToasts, addToast, removeToast } from "./toast";

describe("Toast Library", () => {
  beforeEach(() => {
    clearToasts();
  });

  it("adds info, success, warning, and error toasts", () => {
    toast.info("Info message", "Info Title");
    toast.success("Success message", "Success Title");
    toast.warning("Warning message", "Warning Title");
    toast.error("Error message", "Error Title");

    expect(toasts.value.length).toBe(4);
    expect(toasts.value[0].type).toBe("info");
    expect(toasts.value[0].message).toBe("Info message");
    expect(toasts.value[0].title).toBe("Info Title");

    expect(toasts.value[1].type).toBe("success");
    expect(toasts.value[2].type).toBe("warning");
    expect(toasts.value[3].type).toBe("error");
  });

  it("formats Error instances into message strings", () => {
    const err = new Error("Database connection failed");
    toast.error(err, "DB Error");

    expect(toasts.value.length).toBe(1);
    expect(toasts.value[0].type).toBe("error");
    expect(toasts.value[0].message).toBe("Database connection failed");
    expect(toasts.value[0].title).toBe("DB Error");
  });

  it("removes toasts by id and clears all toasts", () => {
    const id1 = toast.info("Msg 1");
    const id2 = toast.info("Msg 2");
    expect(toasts.value.length).toBe(2);

    removeToast(id1);
    expect(toasts.value.length).toBe(1);
    expect(toasts.value[0].id).toBe(id2);

    clearToasts();
    expect(toasts.value.length).toBe(0);
  });
});
