package mqtt

// The SubscriptionNotifier interface is defined in service/mqtt_service_interface.go.
// The ConnectionManager implements it via OnSubscriptionAdded/OnSubscriptionRemoved,
// and implements BrokerNotifier, defined in the same file, via
// OnBrokerChanged/OnBrokerDeleted.
