package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestDynamoCounterIncrementBuildsAtomicTransaction(t *testing.T) {
	client := &fakeDynamoCounterAPI{}
	store := newDynamoCounterStore(client, "analytics")
	if err := store.Increment(context.Background(), "abcdefghijklmnop", "message-1", time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if client.transaction == nil || len(client.transaction.TransactItems) != 3 {
		t.Fatalf("unexpected transaction: %#v", client.transaction)
	}
}

func TestDynamoCounterTreatsExistingEventAsSuccess(t *testing.T) {
	client := &fakeDynamoCounterAPI{
		transactionErr: &types.TransactionCanceledException{},
		getOutput: &dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{
			"metric_key": &types.AttributeValueMemberS{Value: "EVENT#message-1"},
		}},
	}
	store := newDynamoCounterStore(client, "analytics")
	if err := store.Increment(context.Background(), "abcdefghijklmnop", "message-1", time.Now()); err != nil {
		t.Fatalf("duplicate returned error: %v", err)
	}
}

func TestDynamoCounterDoesNotHideUnknownCancellation(t *testing.T) {
	want := &types.TransactionCanceledException{}
	client := &fakeDynamoCounterAPI{transactionErr: want, getOutput: &dynamodb.GetItemOutput{}}
	store := newDynamoCounterStore(client, "analytics")
	if err := store.Increment(context.Background(), "abcdefghijklmnop", "message-1", time.Now()); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

func TestDynamoCounterSnapshotPaginates(t *testing.T) {
	client := &fakeDynamoCounterAPI{scanOutputs: []*dynamodb.ScanOutput{
		{
			Items: []map[string]types.AttributeValue{
				{"metric_key": &types.AttributeValueMemberS{Value: "TOTAL"}, "count": &types.AttributeValueMemberN{Value: "3"}},
			},
			LastEvaluatedKey: map[string]types.AttributeValue{"metric_key": &types.AttributeValueMemberS{Value: "TOTAL"}},
		},
		{Items: []map[string]types.AttributeValue{
			{"metric_key": &types.AttributeValueMemberS{Value: "URL#abcdefghijklmnop"}, "count": &types.AttributeValueMemberN{Value: "3"}},
		}},
	}}
	stats, err := newDynamoCounterStore(client, "analytics").Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalRedirects != 3 || stats.RedirectsByShortURL["abcdefghijklmnop"] != 3 || client.scanCalls != 2 {
		t.Fatalf("unexpected snapshot: %#v, calls=%d", stats, client.scanCalls)
	}
}

type fakeDynamoCounterAPI struct {
	transaction    *dynamodb.TransactWriteItemsInput
	transactionErr error
	getOutput      *dynamodb.GetItemOutput
	getErr         error
	scanOutputs    []*dynamodb.ScanOutput
	scanErr        error
	scanCalls      int
	describeErr    error
}

func (f *fakeDynamoCounterAPI) TransactWriteItems(_ context.Context, input *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	f.transaction = input
	return &dynamodb.TransactWriteItemsOutput{}, f.transactionErr
}

func (f *fakeDynamoCounterAPI) GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if f.getOutput == nil {
		f.getOutput = &dynamodb.GetItemOutput{}
	}
	return f.getOutput, f.getErr
}

func (f *fakeDynamoCounterAPI) Scan(context.Context, *dynamodb.ScanInput, ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	if f.scanErr != nil {
		return nil, f.scanErr
	}
	output := f.scanOutputs[f.scanCalls]
	f.scanCalls++
	return output, nil
}

func (f *fakeDynamoCounterAPI) DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{}, f.describeErr
}
