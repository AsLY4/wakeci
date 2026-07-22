package main

import "testing"

func TestClientIsSubscribed(t *testing.T) {
	tests := []struct {
		name          string
		subscriptions []string
		tag           string
		want          bool
		wantIndex     int
	}{
		{
			name:          "exact build",
			subscriptions: []string{"build:update:1"},
			tag:           "build:update:1",
			want:          true,
		},
		{
			name:          "numeric prefix is not another build",
			subscriptions: []string{"build:update:1"},
			tag:           "build:update:15",
		},
		{
			name:          "exact subscription is not a namespace",
			subscriptions: []string{"build:update:1"},
			tag:           "build:update:1:task",
		},
		{
			name:          "namespace subscription",
			subscriptions: []string{"build:update:"},
			tag:           "build:update:15",
			want:          true,
		},
		{
			name:          "namespace does not cross message types",
			subscriptions: []string{"build:update:"},
			tag:           "build:log:15",
		},
		{
			name:          "returns matching index",
			subscriptions: []string{"build:log:2", "build:update:2"},
			tag:           "build:update:2",
			want:          true,
			wantIndex:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{SubscribedTo: tt.subscriptions}
			got, index := client.IsSubscribed(tt.tag)
			if got != tt.want {
				t.Errorf("IsSubscribed(%q) = %t, want %t", tt.tag, got, tt.want)
			}
			if got && index != tt.wantIndex {
				t.Errorf("matching index = %d, want %d", index, tt.wantIndex)
			}
		})
	}
}

func TestClientUnsubscribeUsesExactSubscription(t *testing.T) {
	client := &Client{
		SubscribedTo: []string{"build:update:"},
		Logger:       L,
	}

	client.Subscribe("build:update:1")
	client.Unsubscribe("build:update:1")

	if len(client.SubscribedTo) != 1 || client.SubscribedTo[0] != "build:update:" {
		t.Errorf("subscriptions = %v, want the broad subscription to remain", client.SubscribedTo)
	}
}
